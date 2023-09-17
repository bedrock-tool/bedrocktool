package worlds

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/scripting"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/gregwebs/go-recovery"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	_ "github.com/df-mc/dragonfly/server/world/biome"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type WorldSettings struct {
	// settings
	VoidGen         bool
	WithPacks       bool
	SaveImage       bool
	SaveEntities    bool
	SaveInventories bool
	BlockUpdates    bool
	ExcludeMobs     []string
	StartPaused     bool
	PreloadReplay   string
	ChunkRadius     int32
	Script          string
}

type serverState struct {
	useOldBiomes  bool
	useHashedRids bool
	worldCounter  int
	WorldName     string
	biomes        map[string]any
	radius        int32

	openItemContainers map[byte]*itemContainer
	playerInventory    []protocol.ItemInstance
	packs              []utils.Pack

	Name string
}

type worldsHandler struct {
	wg    sync.WaitGroup
	ctx   context.Context
	proxy *proxy.Context
	mapUI *MapUI
	ui    ui.UI
	bp    *behaviourpack.BehaviourPack

	scripting *scripting.VM

	// lock used for when the worldState gets swapped
	worldState     *worldState
	worldStateLock sync.Mutex

	serverState  serverState
	settings     WorldSettings
	customBlocks []protocol.BlockEntry
	doNotRemove  []int64
}

func resetGlobals() {
	world.ClearStates()
	world.LoadBlockStates()
	block.InitBlocks()
	world.ResetBiomes()
}

func NewWorldsHandler(ui ui.UI, settings WorldSettings) *proxy.Handler {
	settings.ExcludeMobs = slices.DeleteFunc(settings.ExcludeMobs, func(mob string) bool {
		return mob == ""
	})

	if settings.ChunkRadius == 0 {
		settings.ChunkRadius = 80
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &worldsHandler{
		ctx: ctx,
		ui:  ui,
		serverState: serverState{
			useOldBiomes:       false,
			worldCounter:       0,
			openItemContainers: make(map[byte]*itemContainer),
		},
		settings: settings,
	}
	if settings.StartPaused {
		w.worldState.PauseCapture()
	}
	w.mapUI = NewMapUI(w)
	w.scripting = scripting.New()

	h := &proxy.Handler{
		Name: "Worlds",
		ProxyRef: func(pc *proxy.Context) {
			w.proxy = pc

			w.proxy.AddCommand(func(cmdline []string) bool {
				return w.setWorldName(strings.Join(cmdline, " "), false)
			}, protocol.Command{
				Name:        "setname",
				Description: locale.Loc("setname_desc", nil),
			})

			w.proxy.AddCommand(func(cmdline []string) bool {
				return w.setVoidGen(!w.worldState.VoidGen, false)
			}, protocol.Command{
				Name:        "void",
				Description: locale.Loc("void_desc", nil),
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.settings.ExcludeMobs = append(w.settings.ExcludeMobs, s...)
				w.proxy.SendMessage(fmt.Sprintf("Exluding: %s", strings.Join(w.settings.ExcludeMobs, ", ")))
				return true
			}, protocol.Command{
				Name:        "exclude-mob",
				Description: "add a mob to the list of mobs to ignore",
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.worldState.PauseCapture()
				w.proxy.SendMessage("Paused Capturing")
				return true
			}, protocol.Command{
				Name:        "stop-capture",
				Description: "stop capturing entities, chunks",
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.proxy.SendMessage("Restarted Capturing")
				pos := cube.Pos{int(math.Floor(float64(w.proxy.Player.Position[0]))), int(math.Floor(float64(w.proxy.Player.Position[1]))), int(math.Floor(float64(w.proxy.Player.Position[2])))}
				w.worldState.UnpauseCapture(pos, w.serverState.radius, func(cp world.ChunkPos, c *chunk.Chunk) {
					w.mapUI.SetChunk(cp, c, false)
				})
				return true
			}, protocol.Command{
				Name:        "start-capture",
				Description: "start capturing entities, chunks",
			})

			w.proxy.AddCommand(func(s []string) bool {
				go recovery.Go(func() error {
					return w.SaveAndReset(false, nil)
				})
				return true
			}, protocol.Command{
				Name:        "save-world",
				Description: "immediately save and reset the world state",
			})
		},

		AddressAndName: func(address, hostname string) error {
			w.bp = behaviourpack.New(hostname)
			w.serverState.Name = hostname
			w.newWorldState()

			if settings.Script != "" {
				err := w.scripting.Load(settings.Script)
				if err != nil {
					return err
				}
			}

			err := w.preloadReplay()
			if err != nil {
				return err
			}

			return nil
		},

		ToClientGameDataModifier: func(gd *minecraft.GameData) {
			gd.ClientSideGeneration = false
		},

		ConnectCB: func() bool {
			w.ui.Message(messages.SetUIState(messages.UIStateMain))

			w.ui.Message(messages.SetWorldName{
				WorldName: w.worldState.Name,
			})

			w.proxy.ClientWritePacket(&packet.ChunkRadiusUpdated{
				ChunkRadius: w.settings.ChunkRadius,
			})

			w.proxy.Server.WritePacket(&packet.RequestChunkRadius{
				ChunkRadius: w.settings.ChunkRadius,
			})

			gd := w.proxy.Server.GameData()
			mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
			MapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
			if gd.ServerAuthoritativeInventory {
				MapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
			}

			w.serverState.packs = utils.GetPacks(w.proxy.Server)

			w.proxy.SendMessage(locale.Loc("use_setname", nil))
			w.mapUI.Start(ctx)
			return false
		},

		PacketCB: w.packetCB,
		OnEnd: func() {
			w.wg.Add(1)
			go recovery.Go(func() error {
				defer w.wg.Done()
				return w.SaveAndReset(true, nil)
			})
			w.wg.Wait()
			resetGlobals()
		},
		Deferred: cancel,
	}

	return h
}

func (w *worldsHandler) packetCB(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	switch pk := pk.(type) {
	case *packet.RequestChunkRadius:
		pk.ChunkRadius = w.settings.ChunkRadius
	case *packet.ChunkRadiusUpdated:
		w.serverState.radius = pk.ChunkRadius
		pk.ChunkRadius = w.settings.ChunkRadius
	case *packet.SetTime:
		w.worldState.timeSync = time.Now()
		w.worldState.time = int(pk.Time)
	case *packet.StartGame:
		w.worldState.timeSync = time.Now()
		w.worldState.time = int(pk.Time)
		w.serverState.useHashedRids = pk.UseBlockNetworkIDHashes
		if w.serverState.useHashedRids {
			return nil, errors.New("this server uses the new hashed block id system, this hasnt been implemented yet, sorry")
		}

		world.InsertCustomItems(pk.Items)
		for _, ie := range pk.Items {
			w.bp.AddItem(ie)
		}
		if len(pk.Blocks) > 0 {
			logrus.Info(locale.Loc("using_customblocks", nil))
			for _, be := range pk.Blocks {
				w.bp.AddBlock(be)
			}
			// telling the chunk code what custom blocks there are so it can generate offsets
			world.InsertCustomBlocks(pk.Blocks)
			w.customBlocks = pk.Blocks
		}

		w.serverState.WorldName = pk.WorldName
		if pk.WorldName != "" {
			w.worldState.Name = pk.WorldName
		}

		{ // check game version
			gv := strings.Split(pk.BaseGameVersion, ".")
			var err error
			if len(gv) > 1 {
				var ver int
				ver, err = strconv.Atoi(gv[1])
				w.serverState.useOldBiomes = ver < 18
			}
			if err != nil || len(gv) <= 1 {
				logrus.Info(locale.Loc("guessing_version", nil))
			}

			dimensionID := pk.Dimension
			if w.serverState.useOldBiomes {
				logrus.Info(locale.Loc("using_under_118", nil))
				if dimensionID == 0 {
					dimensionID += 10
				}
			}
			w.worldState.dimension, _ = world.DimensionByID(int(dimensionID))
		}
		err := w.openWorldState(w.worldState.dimension, w.settings.StartPaused)
		if err != nil {
			return nil, err
		}

	case *packet.ItemComponent:
		w.bp.ApplyComponentEntries(pk.Items)
	case *packet.BiomeDefinitionList:
		err := nbt.UnmarshalEncoding(pk.SerialisedBiomeDefinitions, &w.serverState.biomes, nbt.NetworkLittleEndian)
		if err != nil {
			logrus.Error(err)
		}
		for k, v := range w.serverState.biomes {
			_, ok := world.BiomeByName(k)
			if !ok {
				world.RegisterBiome(&customBiome{
					name: k,
					data: v.(map[string]any),
				})
			}
		}
		w.bp.AddBiomes(w.serverState.biomes)
	}

	forward := true
	pk = w.handleItemPackets(pk, &forward)
	pk = w.handleMapPackets(pk, &forward, toServer)
	pk = w.handleChunkPackets(pk)
	pk = w.handleEntityPackets(pk)

	if !forward {
		return nil, nil
	}
	return pk, nil
}

func (w *worldsHandler) preloadReplay() error {
	if w.settings.PreloadReplay == "" {
		return nil
	}
	var conn minecraft.IConn
	var err error
	conn, err = proxy.CreateReplayConnector(context.Background(), w.settings.PreloadReplay, func(header packet.Header, payload []byte, src, dst net.Addr) {
		pk, ok := proxy.DecodePacket(header, payload)
		if !ok {
			logrus.Error("unknown packet", header)
			return
		}

		if header.PacketID == packet.IDCommandRequest {
			return
		}

		toServer := src.String() == conn.LocalAddr().String()
		_, err := w.packetCB(pk, toServer, time.Now(), false)
		if err != nil {
			logrus.Error(err)
		}
	}, func() {}, func(p *resource.Pack) {})
	if err != nil {
		return err
	}
	w.proxy.Server = conn
	for {
		_, err := conn.ReadPacket()
		if err != nil {
			break
		}
	}
	w.proxy.Server = nil

	logrus.Info("finished preload")
	resetGlobals()
	return nil
}

func (w *worldsHandler) setVoidGen(val bool, fromUI bool) bool {
	w.worldState.VoidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.worldState.VoidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.proxy.SendMessage(s)

	if !fromUI {
		w.ui.Message(messages.SetVoidGen{
			Value: w.worldState.VoidGen,
		})
	}

	return true
}

func (w *worldsHandler) setWorldName(val string, fromUI bool) bool {
	err := w.renameWorldState(val)
	if err != nil {
		logrus.Error(err)
		return false
	}
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.worldState.Name}))

	if !fromUI {
		w.ui.Message(messages.SetWorldName{
			WorldName: w.worldState.Name,
		})
	}

	return true
}

func (w *worldsHandler) defaultName() string {
	worldName := "world"
	if w.serverState.worldCounter > 0 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	} else if w.serverState.WorldName != "" {
		worldName = w.serverState.WorldName
	}
	return worldName
}

func (w *worldsHandler) SaveAndReset(end bool, dim world.Dimension) (err error) {
	// replacing the current world state if it needs to be reset
	w.worldStateLock.Lock()
	if dim == nil {
		dim = w.worldState.dimension
	}

	if len(w.worldState.storedChunks) == 0 {
		w.reset(dim)
		w.worldStateLock.Unlock()
		return nil
	}

	if w.settings.SaveImage {
		f, _ := os.Create(w.worldState.folder + ".png")
		png.Encode(f, w.mapUI.ToImage())
		f.Close()
	}

	w.wg.Add(1)
	defer w.wg.Done()
	worldState := w.worldState
	w.serverState.worldCounter += 1
	if !end {
		w.reset(dim)
	} else {
		w.worldState = nil
	}
	w.worldStateLock.Unlock()
	// do not access w.worldState after this
	worldState.excludeMobs = w.settings.ExcludeMobs

	playerPos := w.proxy.Player.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	text := locale.Loc("saving_world", locale.Strmap{"Name": worldState.Name, "Count": len(worldState.storedChunks)})
	logrus.Info(text)
	w.proxy.SendMessage(text)

	filename := worldState.folder + ".mcworld"

	w.ui.Message(messages.SavingWorld{
		World: &messages.SavedWorld{
			Name:     worldState.Name,
			Path:     filename,
			Chunks:   len(worldState.storedChunks),
			Entities: len(worldState.state.entities),
		},
	})

	err = worldState.Finish(w.playerData(), spawnPos, w.proxy.Server.GameData(), w.bp)
	if err != nil {
		logrus.Error(err)
		return err
	}
	w.AddPacks(worldState.folder)

	// zip it
	err = utils.ZipFolder(filename, worldState.folder)
	if err != nil {
		logrus.Error(err)
		return err
	}
	logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
	//os.RemoveAll(folder)
	return err
}

func (w *worldsHandler) reset(dim world.Dimension) error {
	// carry over deffered and dim from previous
	var deferred bool
	if w.worldState != nil {
		deferred = w.worldState.useDeferred
	}

	w.newWorldState()
	err := w.openWorldState(dim, deferred)
	if err != nil {
		return err
	}

	w.mapUI.Reset()
	return nil
}

func (w *worldsHandler) newWorldState() {
	var err error
	w.worldState, err = newWorldState(func(cp world.ChunkPos, c *chunk.Chunk) {
		w.mapUI.SetChunk(cp, c, false)
	})
	if err != nil {
		logrus.Error(err)
		return
	}
	w.worldState.VoidGen = w.settings.VoidGen
}

func (w *worldsHandler) openWorldState(dim world.Dimension, deferred bool) error {
	name := w.defaultName()
	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, name)
	err := w.worldState.Open(name, folder, dim, deferred)
	if err != nil {
		return err
	}
	return nil
}

func (w *worldsHandler) renameWorldState(name string) error {
	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, name)
	return w.worldState.Rename(name, folder)
}
