package worlds

import (
	"errors"
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	_ "github.com/df-mc/dragonfly/server/world/biome"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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
}

type serverState struct {
	useOldBiomes  bool
	useHashedRids bool
	worldCounter  int
	WorldName     string
	biomes        map[string]any
	radius        int32

	playerInventory []protocol.ItemInstance
	packs           []utils.Pack

	Name string
}

type worldsHandler struct {
	wg    sync.WaitGroup
	proxy *proxy.Context
	mapUI *MapUI
	ui    ui.UI
	bp    *behaviourpack.BehaviourPack

	worldState   *worldState
	serverState  serverState
	settings     WorldSettings
	customBlocks []protocol.BlockEntry
}

func NewWorldsHandler(ui ui.UI, settings WorldSettings) *proxy.Handler {
	settings.ExcludeMobs = slices.DeleteFunc(settings.ExcludeMobs, func(mob string) bool {
		return mob == ""
	})

	w := &worldsHandler{
		ui: ui,
		serverState: serverState{
			useOldBiomes: false,
			worldCounter: 0,
		},
		settings: settings,
	}
	if settings.StartPaused {
		w.worldState.PauseCapture()
	}
	w.mapUI = NewMapUI(w)

	return &proxy.Handler{
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
				w.SaveAndReset()
				return true
			}, protocol.Command{
				Name:        "save-world",
				Description: "immediately save and reset the world state",
			})

		},
		AddressAndName: func(address, hostname string) error {
			w.bp = behaviourpack.New(hostname)
			w.serverState.Name = hostname
			w.reset()
			return nil
		},
		OnClientConnect: func(conn minecraft.IConn) {
			w.ui.Message(messages.SetUIState(messages.UIStateConnecting))
		},

		ToClientGameDataModifier: func(gd *minecraft.GameData) {
			gd.ClientSideGeneration = false
		},

		ConnectCB: func(err error) bool {
			if err != nil {
				return true
			}

			w.ui.Message(messages.SetWorldName{
				WorldName: w.worldState.Name,
			})
			w.ui.Message(messages.SetUIState(messages.UIStateMain))

			w.proxy.ClientWritePacket(&packet.ChunkRadiusUpdated{
				ChunkRadius: 80,
			})

			gd := w.proxy.Server.GameData()
			mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
			MapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
			if gd.ServerAuthoritativeInventory {
				MapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
			}

			w.serverState.packs = utils.GetPacks(w.proxy.Server)

			w.proxy.SendMessage(locale.Loc("use_setname", nil))
			w.mapUI.Start()
			return false
		},

		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			switch pk := pk.(type) {
			case *packet.ChunkRadiusUpdated:
				w.serverState.radius = pk.ChunkRadius
				pk.ChunkRadius = 80
			case *packet.SetTime:
				w.worldState.timeSync = time.Now()
				w.worldState.time = int(pk.Time)
			case *packet.StartGame:
				w.worldState.timeSync = time.Now()
				w.worldState.time = int(pk.Time)
				w.worldState.dimension, _ = world.DimensionByID(int(pk.Dimension))
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
		},
		OnEnd: func() {
			w.SaveAndReset()
			w.wg.Wait()
			world.ResetStates()
			world.ResetBiomes()
		},
	}
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
	w.worldState.Name = val
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

func (w *worldsHandler) SaveAndReset() {
	if len(w.worldState.storedChunks) == 0 {
		w.reset()
		return
	}

	playerPos := w.proxy.Player.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	if w.settings.SaveImage {
		f, _ := os.Create(w.worldState.folder + ".png")
		png.Encode(f, w.mapUI.ToImage())
		f.Close()
	}

	text := locale.Loc("saving_world", locale.Strmap{"Name": w.worldState.Name, "Count": len(w.worldState.storedChunks)})
	logrus.Info(text)
	w.proxy.SendMessage(text)

	filename := w.worldState.folder + ".mcworld"

	w.ui.Message(messages.SavingWorld{
		World: &messages.SavedWorld{
			Name:   w.worldState.Name,
			Path:   filename,
			Chunks: len(w.worldState.storedChunks),
		},
	})

	w.wg.Add(1)
	w.worldState.excludeMobs = w.settings.ExcludeMobs
	worldState := w.worldState
	w.serverState.worldCounter += 1
	w.reset()

	go func() {
		defer w.wg.Done()
		err := worldState.Finish(w.playerData(), spawnPos, w.proxy.Server.GameData(), w.bp)
		if err != nil {
			logrus.Error(err)
			return
		}
		w.AddPacks(worldState.folder)

		// zip it
		err = utils.ZipFolder(filename, worldState.folder)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
		//os.RemoveAll(folder)
		w.ui.Message(messages.SetUIState(messages.UIStateMain))
	}()
}

func (w *worldsHandler) reset() {
	// carry over deffered and dim from previous
	var dim world.Dimension
	var deferred bool
	if w.worldState != nil {
		dim = w.worldState.dimension
		deferred = w.worldState.useDeferred
	}

	// create folder
	name := w.defaultName()
	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, name)
	os.MkdirAll(folder, 0777)

	var err error
	w.worldState, err = newWorldState(name, folder, dim)
	if err != nil {
		logrus.Error(err)
		return
	}
	w.worldState.VoidGen = w.settings.VoidGen
	if deferred {
		w.worldState.PauseCapture()
	}
	w.mapUI.Reset()
}
