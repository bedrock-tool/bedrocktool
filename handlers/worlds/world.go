package worlds

import (
	"archive/zip"
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
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/bedrock-tool/bedrocktool/utils/resourcepack"
	"github.com/flytam/filenamify"
	"github.com/google/uuid"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	_ "github.com/df-mc/dragonfly/server/world/biome"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type WorldSettings struct {
	VoidGen         bool
	SaveImage       bool
	SaveEntities    bool
	SaveInventories bool
	ExcludedMobs    []string
	StartPaused     bool
	PreloadReplay   string
	ChunkRadius     int32
	Script          string
	Players         bool
	BlockUpdates    bool
}

type serverState struct {
	useOldBiomes  bool
	useHashedRids bool
	haveStartGame bool
	worldCounter  int
	WorldName     string
	radius        int32

	biomes *world.BiomeRegistry
	blocks *world.BlockRegistryImpl

	behaviorPack *behaviourpack.Pack
	resourcePack *resourcepack.Pack

	customBlocks       []protocol.BlockEntry
	openItemContainers map[byte]*itemContainer
	playerInventory    []protocol.ItemInstance
	dimensions         map[int]protocol.DimensionDefinition
	playerSkins        map[uuid.UUID]*protocol.Skin

	Name string
}

type worldsHandler struct {
	wg      sync.WaitGroup
	ctx     context.Context
	session *proxy.Session
	mapUI   *MapUI
	log     *logrus.Entry

	scripting *scripting.VM

	// lock used for when the worldState gets swapped
	currentWorld   *worldstate.World
	worldStateLock sync.Mutex

	serverState serverState
	settings    WorldSettings
}

type itemContainer struct {
	OpenPacket *packet.ContainerOpen
	Content    *packet.InventoryContent
}

func NewWorldsHandler(settings WorldSettings) *proxy.Handler {
	settings.ExcludedMobs = slices.DeleteFunc(settings.ExcludedMobs, func(mob string) bool {
		return mob == ""
	})

	if settings.ChunkRadius == 0 {
		settings.ChunkRadius = 80
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &worldsHandler{
		ctx:      ctx,
		log:      logrus.WithField("part", "WorldsHandler"),
		settings: settings,
	}

	h := &proxy.Handler{
		Name: "Worlds",

		SessionStart: func(session *proxy.Session, serverName string) error {
			w.session = session
			w.currentWorld = nil
			w.serverState = serverState{
				useOldBiomes:       false,
				worldCounter:       0,
				openItemContainers: make(map[byte]*itemContainer),
				dimensions:         make(map[int]protocol.DimensionDefinition),
				playerSkins:        make(map[uuid.UUID]*protocol.Skin),
				biomes:             world.DefaultBiomes.Clone(),
			}

			w.mapUI = NewMapUI(w)
			w.scripting = scripting.New()

			w.session.AddCommand(func(cmdline []string) bool {
				return w.setWorldName(strings.Join(cmdline, " "))
			}, protocol.Command{
				Name:        "setname",
				Description: locale.Loc("setname_desc", nil),
			})

			w.session.AddCommand(func(cmdline []string) bool {
				return w.setVoidGen(!w.currentWorld.VoidGen)
			}, protocol.Command{
				Name:        "void",
				Description: locale.Loc("void_desc", nil),
			})

			w.session.AddCommand(func(s []string) bool {
				w.settings.ExcludedMobs = append(w.settings.ExcludedMobs, s...)
				w.session.SendMessage(fmt.Sprintf("Exluding: %s", strings.Join(w.settings.ExcludedMobs, ", ")))
				return true
			}, protocol.Command{
				Name:        "exclude-mob",
				Description: "add a mob to the list of mobs to ignore",
			})

			w.session.AddCommand(func(s []string) bool {
				w.currentWorld.PauseCapture()
				w.session.SendMessage("Paused Capturing")
				return true
			}, protocol.Command{
				Name:        "stop-capture",
				Description: "stop capturing entities, chunks",
			})

			w.session.AddCommand(func(s []string) bool {
				w.session.SendMessage("Restarted Capturing")
				pos := cube.Pos{
					int(math.Floor(float64(w.session.Player.Position[0]))),
					int(math.Floor(float64(w.session.Player.Position[1]))),
					int(math.Floor(float64(w.session.Player.Position[2]))),
				}
				w.currentWorld.UnpauseCapture(pos, w.serverState.radius)
				return true
			}, protocol.Command{
				Name:        "start-capture",
				Description: "start capturing entities, chunks",
			})

			w.session.AddCommand(func(s []string) bool {
				w.SaveAndReset(false, nil)
				return true
			}, protocol.Command{
				Name:        "save-world",
				Description: "immediately save and reset the world state",
			})

			w.serverState.behaviorPack = behaviourpack.New(serverName)
			w.serverState.resourcePack = resourcepack.New()
			w.serverState.Name = serverName

			// initialize a worldstate
			var err error
			w.currentWorld, err = worldstate.New(w.serverState.dimensions, w.mapUI.SetChunk)
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

		GameDataModifier: func(gd *minecraft.GameData) {
			gd.ClientSideGeneration = false
		},

		OnConnect: func() bool {
			messages.Router.Handle(&messages.Message{
				Source: "subcommand",
				Target: "ui",
				Data:   messages.UIStateMain,
			})

			messages.Router.Handle(&messages.Message{
				Source: "subcommand",
				Target: "ui",
				Data: messages.SetValue{
					Name:  "worldName",
					Value: w.currentWorld.Name,
				},
			})

			w.session.ClientWritePacket(&packet.ChunkRadiusUpdated{
				ChunkRadius: w.settings.ChunkRadius,
			})

			w.session.Server.WritePacket(&packet.RequestChunkRadius{
				ChunkRadius: w.settings.ChunkRadius,
			})

			gd := w.session.Server.GameData()
			mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
			mapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
			if gd.ServerAuthoritativeInventory {
				mapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
			}

			w.session.SendMessage(locale.Loc("use_setname", nil))
			w.mapUI.Start(ctx)
			return false
		},

		PacketCallback: w.packetCB,
		OnSessionEnd: func() {
			w.SaveAndReset(true, nil)
			w.wg.Wait()
		},
		OnProxyEnd: cancel,
	}

	return h
}

func (w *worldsHandler) preloadReplay() error {
	if w.settings.PreloadReplay == "" {
		return nil
	}
	log := w.log.WithField("func", "preloadReplay")
	var conn *proxy.ReplayConnector
	var err error
	conn, err = proxy.CreateReplayConnector(context.Background(), w.settings.PreloadReplay, func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
		pk, ok := proxy.DecodePacket(header, payload, conn.ShieldID())
		if !ok {
			log.Error("unknown packet", header)
			return
		}

		if header.PacketID == packet.IDCommandRequest {
			return
		}

		toServer := src.String() == conn.LocalAddr().String()
		_, err := w.packetCB(pk, toServer, time.Now(), false)
		if err != nil {
			log.Error(err)
		}
	}, nil)
	if err != nil {
		return err
	}
	w.session.Server = conn

	err = conn.ReadUntilLogin()
	if err != nil {
		return err
	}

	for {
		_, err := conn.ReadPacket()
		if err != nil {
			break
		}
	}
	w.session.Server = nil

	log.Info("finished preload")
	w.serverState.blocks = nil
	return nil
}

func (w *worldsHandler) setVoidGen(val bool) bool {
	w.currentWorld.VoidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.currentWorld.VoidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.session.SendMessage(s)

	var voidGen = "false"
	if w.currentWorld.VoidGen {
		voidGen = "true"
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.SetValue{
			Name:  "voidGen",
			Value: voidGen,
		},
	})

	return true
}

func (w *worldsHandler) setWorldName(val string) bool {
	err := w.renameWorldState(val)
	if err != nil {
		w.log.Error(err)
		return false
	}
	w.session.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.currentWorld.Name}))

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.SetValue{
			Name:  "worldName",
			Value: w.currentWorld.Name,
		},
	})

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
		err := w.saveWorldState(worldState)
		if err != nil {
			w.log.Error(err)
		}
	}()
}

func (w *worldsHandler) saveWorldState(worldState *worldstate.World) error {
	playerPos := w.session.Player.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	text := locale.Loc("saving_world", locale.Strmap{"Name": worldState.Name, "Count": len(worldState.StoredChunks)})
	w.log.Info(text)
	w.session.SendMessage(text)

	filename := worldState.Folder + ".mcworld"

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  worldState.Name,
			State: "Saving",
		},
	})
	err := worldState.Finish(w.playerData(), w.settings.ExcludedMobs, w.settings.Players, spawnPos, w.session.Server.GameData(), w.serverState.behaviorPack)
	if err != nil {
		return err
	}

	err = worldState.FinalizePacks(w.serverState.Name)
	if err != nil {
		return err
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  worldState.Name,
			State: "Writing mcworld file",
		},
	})

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	utils.ZipCompressPool(zw)
	err = zw.AddFS(os.DirFS(worldState.Folder))
	if err != nil {
		return err
	}
	err = zw.Close()
	if err != nil {
		return err
	}

	w.log.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.FinishedSavingWorld{
			World: &messages.SavedWorld{
				Name:     worldState.Name,
				Path:     filename,
				Chunks:   len(worldState.StoredChunks),
				Entities: worldState.EntityCount(),
			},
		},
	})

	return nil
}

func (w *worldsHandler) reset(dim world.Dimension) (err error) {
	// create new world state
	w.currentWorld, err = worldstate.New(w.serverState.dimensions, w.mapUI.SetChunk)
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
	serverName, _ := filenamify.FilenamifyV2(w.serverState.Name)
	folder := fmt.Sprintf("worlds/%s/%s", serverName, name)
	w.currentWorld.BiomeRegistry = w.serverState.biomes
	w.currentWorld.BlockRegistry = w.serverState.blocks
	w.currentWorld.BehaviorPack = w.serverState.behaviorPack
	w.currentWorld.ResourcePacks = w.session.Server.ResourcePacks()
	w.currentWorld.Open(name, folder, deferred)
}

func (w *worldsHandler) renameWorldState(name string) error {
	serverName, _ := filenamify.FilenamifyV2(w.serverState.Name)
	folder := fmt.Sprintf("worlds/%s/%s", serverName, name)
	return w.currentWorld.Rename(name, folder)
}
