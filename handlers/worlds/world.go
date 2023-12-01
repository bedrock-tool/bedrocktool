package worlds

import (
	"context"
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/scripting"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	_ "github.com/df-mc/dragonfly/server/world/biome"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type WorldSettings struct {
	VoidGen         bool
	WithPacks       bool
	SaveImage       bool
	SaveEntities    bool
	SaveInventories bool
	ExcludedMobs    []string
	StartPaused     bool
	PreloadReplay   string
	ChunkRadius     int32
	Script          string
}

type serverState struct {
	useOldBiomes  bool
	useHashedRids bool
	haveStartGame bool
	worldCounter  int
	WorldName     string
	biomes        map[string]any
	radius        int32

	openItemContainers map[byte]*itemContainer
	playerInventory    []protocol.ItemInstance
	packs              []utils.Pack
	dimensions         map[int]protocol.DimensionDefinition

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
	currentWorld   *worldstate.World
	worldStateLock sync.Mutex

	serverState  serverState
	settings     WorldSettings
	blockStates  []world.BlockState
	customBlocks []protocol.BlockEntry
	err          chan error
}

type itemContainer struct {
	OpenPacket *packet.ContainerOpen
	Content    *packet.InventoryContent
}

func resetGlobals() {
	world.ClearStates()
	world.LoadBlockStates()
	block.InitBlocks()
	world.ResetBiomes()
}

func NewWorldsHandler(ui ui.UI, settings WorldSettings) *proxy.Handler {
	settings.ExcludedMobs = slices.DeleteFunc(settings.ExcludedMobs, func(mob string) bool {
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
			dimensions:         make(map[int]protocol.DimensionDefinition),
		},
		settings: settings,
		err:      make(chan error),
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
				return w.setVoidGen(!w.currentWorld.VoidGen, false)
			}, protocol.Command{
				Name:        "void",
				Description: locale.Loc("void_desc", nil),
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.settings.ExcludedMobs = append(w.settings.ExcludedMobs, s...)
				w.proxy.SendMessage(fmt.Sprintf("Exluding: %s", strings.Join(w.settings.ExcludedMobs, ", ")))
				return true
			}, protocol.Command{
				Name:        "exclude-mob",
				Description: "add a mob to the list of mobs to ignore",
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.currentWorld.PauseCapture()
				w.proxy.SendMessage("Paused Capturing")
				return true
			}, protocol.Command{
				Name:        "stop-capture",
				Description: "stop capturing entities, chunks",
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.proxy.SendMessage("Restarted Capturing")
				pos := cube.Pos{int(math.Floor(float64(w.proxy.Player.Position[0]))), int(math.Floor(float64(w.proxy.Player.Position[1]))), int(math.Floor(float64(w.proxy.Player.Position[2])))}
				w.currentWorld.UnpauseCapture(pos, w.serverState.radius, func(cp world.ChunkPos, c *chunk.Chunk) {
					w.mapUI.SetChunk(cp, c, false)
				})
				return true
			}, protocol.Command{
				Name:        "start-capture",
				Description: "start capturing entities, chunks",
			})

			w.proxy.AddCommand(func(s []string) bool {
				w.SaveAndReset(false, nil)
				return true
			}, protocol.Command{
				Name:        "save-world",
				Description: "immediately save and reset the world state",
			})
		},

		AddressAndName: func(address, hostname string) (err error) {
			w.bp = behaviourpack.New(hostname)
			w.serverState.Name = hostname

			// initialize a worldstate
			w.currentWorld, err = worldstate.New(w.chunkCB, w.serverState.dimensions)
			if err != nil {
				return err
			}
			w.currentWorld.VoidGen = w.settings.VoidGen
			if settings.StartPaused {
				w.currentWorld.PauseCapture()
			}

			if settings.Script != "" {
				err := w.scripting.Load(settings.Script)
				if err != nil {
					return err
				}
			}

			err = w.preloadReplay()
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
				WorldName: w.currentWorld.Name,
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
			w.SaveAndReset(true, nil)
			w.wg.Wait()
			resetGlobals()
		},
		Deferred: cancel,
	}

	return h
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
	w.currentWorld.VoidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.currentWorld.VoidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.proxy.SendMessage(s)

	if !fromUI {
		w.ui.Message(messages.SetVoidGen{
			Value: w.currentWorld.VoidGen,
		})
	}

	return true
}

func (w *worldsHandler) setWorldName(val string, fromUI bool) bool {
	err := w.renameWorldState(val)
	if err != nil {
		w.err <- err
		return false
	}
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.currentWorld.Name}))

	if !fromUI {
		w.ui.Message(messages.SetWorldName{
			WorldName: w.currentWorld.Name,
		})
	}

	return true
}

func (w *worldsHandler) SaveAndReset(end bool, dim world.Dimension) {
	// replacing the current world state if it needs to be reset
	w.worldStateLock.Lock()
	if dim == nil {
		dim = w.currentWorld.Dimension()
	}

	// if empty just reset and dont save anything
	if len(w.currentWorld.StoredChunks) == 0 {
		if end {
			w.currentWorld = nil
		} else {
			w.reset(dim)
		}
		w.worldStateLock.Unlock()
		return
	}

	// save image of the map
	if w.settings.SaveImage {
		f, _ := os.Create(w.currentWorld.Folder + ".png")
		png.Encode(f, w.mapUI.ToImage())
		f.Close()
	}

	// reset map, increase counter for
	w.serverState.worldCounter += 1
	w.mapUI.Reset()

	// swap states
	worldState := w.currentWorld
	if end {
		w.currentWorld = nil
	} else {
		w.reset(dim)
	}
	w.worldStateLock.Unlock()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.saveWorldState(worldState)
	}()
}

func (w *worldsHandler) saveWorldState(worldState *worldstate.World) {
	playerPos := w.proxy.Player.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	text := locale.Loc("saving_world", locale.Strmap{"Name": worldState.Name, "Count": len(worldState.StoredChunks)})
	logrus.Info(text)
	w.proxy.SendMessage(text)

	filename := worldState.Folder + ".mcworld"

	w.ui.Message(messages.SavingWorld{
		World: &messages.SavedWorld{
			Name:     worldState.Name,
			Path:     filename,
			Chunks:   len(worldState.StoredChunks),
			Entities: len(worldState.StoredChunks),
		},
	})

	err := worldState.Finish(w.playerData(), w.settings.ExcludedMobs, spawnPos, w.proxy.Server.GameData(), w.bp)
	if err != nil {
		w.err <- err
		return
	}
	w.AddPacks(worldState.Folder)

	// zip it
	err = utils.ZipFolder(filename, worldState.Folder)
	if err != nil {
		w.err <- err
		return
	}
	logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
}

func (w *worldsHandler) chunkCB(cp world.ChunkPos, c *chunk.Chunk) {
	w.mapUI.SetChunk(cp, c, false)
}

func (w *worldsHandler) reset(dim world.Dimension) (err error) {
	// create new world state
	w.currentWorld, err = worldstate.New(w.chunkCB, w.serverState.dimensions)
	if err != nil {
		return err
	}
	w.currentWorld.VoidGen = w.settings.VoidGen
	w.currentWorld.SetDimension(dim)

	w.openWorldState(false)
	return nil
}

func (w *worldsHandler) defaultWorldName() string {
	worldName := "world"
	if w.serverState.worldCounter > 0 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	} else if w.serverState.WorldName != "" {
		worldName = w.serverState.WorldName
	}
	return worldName
}

func (w *worldsHandler) openWorldState(deferred bool) {
	name := w.defaultWorldName()
	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, name)
	w.currentWorld.Open(name, folder, deferred)
}

func (w *worldsHandler) renameWorldState(name string) error {
	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, name)
	return w.currentWorld.Rename(name, folder)
}
