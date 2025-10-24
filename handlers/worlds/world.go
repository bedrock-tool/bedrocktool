package worlds

import (
	"archive/zip"
	"context"
	"fmt"
	"image/png"
	"maps"
	"math/rand"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/scripting"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/bedrock-tool/bedrocktool/utils/resourcepack"
	"github.com/google/uuid"

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
	ChunkRadius     int32
	Script          string
	Players         bool
	BlockUpdates    bool
	EntityCulling   bool
}

type serverState struct {
	serverName      string
	useHashedRids   bool
	haveStartGame   bool
	worldCounter    int
	worldName       string
	realChunkRadius int32

	biomes *world.BiomeRegistry
	blocks world.BlockRegistry

	behaviorPack *behaviourpack.Pack
	resourcePack *resourcepack.Pack

	customBlocks       []protocol.BlockEntry
	openItemContainers map[byte]*itemContainer
	playerInventory    []protocol.ItemInstance
	dimensions         map[int]protocol.DimensionDefinition
	playerSkins        map[uuid.UUID]*protocol.Skin
	entityProperties   map[string][]entity.EntityProperty

	playerProperties map[string]*entity.EntityProperty

	entityRenderDistances []float32
}

func (s *serverState) getEntityRenderDistance() float32 {
	var entityRenderDistance float32 = 0
	if len(s.entityRenderDistances) > 4 {
		var sum float32
		for _, dist := range s.entityRenderDistances {
			sum += dist
		}
		entityRenderDistance = sum / float32(len(s.entityRenderDistances))
	}
	return entityRenderDistance
}

type worldsHandler struct {
	wg      sync.WaitGroup
	ctx     context.Context
	session *proxy.Session
	mapUI   *MapUI
	log     *logrus.Entry

	scripting *scripting.VM

	// lock used for when the worldState gets swapped
	worldStateMu sync.Mutex
	worldState   *worldstate.World

	serverState serverState
	settings    WorldSettings
}

type itemContainer struct {
	OpenPacket *packet.ContainerOpen
	Content    *packet.InventoryContent
}

func NewWorldsHandler(ctx context.Context, settings WorldSettings) func() *proxy.Handler {
	settings.ExcludedMobs = slices.DeleteFunc(settings.ExcludedMobs, func(mob string) bool {
		return mob == ""
	})

	if settings.ChunkRadius == 0 {
		settings.ChunkRadius = 76
	}

	return func() *proxy.Handler {
		w := &worldsHandler{
			ctx:      ctx,
			log:      logrus.WithField("part", "WorldsHandler"),
			settings: settings,
		}

		return &proxy.Handler{
			Name: "Worlds",

			SessionStart: w.onSessionStart,

			GameDataModifier: func(s *proxy.Session, gd *minecraft.GameData) {
				gd.ClientSideGeneration = false
			},

			OnConnect: w.onConnect,

			PacketCallback: w.packetHandler,
			OnSessionEnd: func(s *proxy.Session, wg *sync.WaitGroup) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					w.SaveAndReset(true, nil)
					w.wg.Wait()
				}()
			},

			OnPlayerMove: func(s *proxy.Session) {
				playerPos := s.Player.Position
				w.currentWorld(func(world *worldstate.World) {
					world.PlayerMove(playerPos, w.serverState.getEntityRenderDistance(), 0)
				})
			},
		}
	}
}

func (w *worldsHandler) onSessionStart(session *proxy.Session, serverName string) error {
	w.session = session
	w.serverState = serverState{
		serverName:         serverName,
		worldCounter:       0,
		openItemContainers: make(map[byte]*itemContainer),
		dimensions:         make(map[int]protocol.DimensionDefinition),
		playerSkins:        make(map[uuid.UUID]*protocol.Skin),
		biomes:             world.DefaultBiomes.Clone(),
		entityProperties:   make(map[string][]entity.EntityProperty),
		behaviorPack:       behaviourpack.New(serverName),
		resourcePack:       resourcepack.New(),
		playerProperties:   make(map[string]*entity.EntityProperty),
	}

	w.mapUI = NewMapUI(w)

	w.scripting = nil
	if w.settings.Script != "" {
		w.scripting = scripting.New()
		w.scripting.GetWorld = func() *worldstate.World {
			return w.worldState // locked by calls to the vm
		}
		err := w.scripting.Load(w.settings.Script)
		if err != nil {
			return err
		}
	}

	session.AddCommand(func(cmdline []string) bool {
		return w.setWorldName(strings.Join(cmdline, " "))
	}, protocol.Command{
		Name:        "setname",
		Description: locale.Loc("setname_desc", nil),
	})

	session.AddCommand(func(cmdline []string) bool {
		return w.setVoidGen(w.worldState.VoidGen)
	}, protocol.Command{
		Name:        "void",
		Description: locale.Loc("void_desc", nil),
	})

	session.AddCommand(func(args []string) bool {
		w.settings.ExcludedMobs = append(w.settings.ExcludedMobs, args...)
		session.SendMessage(fmt.Sprintf("Exluding: %s", strings.Join(w.settings.ExcludedMobs, ", ")))
		return true
	}, protocol.Command{
		Name:        "exclude-mob",
		Description: "add a mob to the list of mobs to ignore",
	})

	session.AddCommand(func(args []string) bool {
		w.SaveAndReset(false, nil)
		return true
	}, protocol.Command{
		Name:        "save-world",
		Description: "immediately save and reset the world state",
	})

	// initialize a worldstate
	worldState, err := worldstate.New(w.ctx, w.serverState.dimensions, w.mapUI.SetChunk)
	if err != nil {
		return err
	}
	worldState.VoidGen = w.settings.VoidGen
	w.worldState = worldState
	return nil
}

func (w *worldsHandler) onConnect(_ *proxy.Session) bool {
	messages.SendEvent(&messages.EventSetUIState{
		State: messages.UIStateMain,
	})
	messages.SendEvent(&messages.EventSetValue{
		Name:  "worldName",
		Value: w.worldState.Name,
	})

	w.session.ClientWritePacket(&packet.ChunkRadiusUpdated{
		ChunkRadius: w.settings.ChunkRadius,
	})

	w.session.Server.WritePacket(&packet.RequestChunkRadius{
		ChunkRadius: w.settings.ChunkRadius,
	})

	gameData := w.session.Server.GameData()
	mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
	mapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
	if gameData.ServerAuthoritativeInventory {
		mapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
	}

	w.session.SendMessage(locale.Loc("use_setname", nil))
	w.mapUI.Start(w.ctx)
	return false
}

func (w *worldsHandler) currentWorld(fn func(world *worldstate.World)) {
	w.worldStateMu.Lock()
	fn(w.worldState)
	w.worldStateMu.Unlock()
}

func (w *worldsHandler) setVoidGen(val bool) bool {
	w.currentWorld(func(world *worldstate.World) {
		world.VoidGen = val
	})
	var s = locale.Loc("void_generator_false", nil)
	if val {
		s = locale.Loc("void_generator_true", nil)
	}
	w.session.SendMessage(s)

	var voidGen = "false"
	if val {
		voidGen = "true"
	}

	messages.SendEvent(&messages.EventSetValue{
		Name:  "voidGen",
		Value: voidGen,
	})
	return true
}

func (w *worldsHandler) setWorldName(val string) bool {
	err := w.renameWorldState(val)
	if err != nil {
		w.log.Error(err)
		return false
	}
	w.session.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.worldState.Name}))
	messages.SendEvent(&messages.EventSetValue{
		Name:  "worldName",
		Value: w.worldState.Name,
	})
	return true
}

func (w *worldsHandler) SaveAndReset(end bool, dim world.Dimension) {
	// replacing the current world state if it needs to be reset
	w.worldStateMu.Lock()
	defer w.worldStateMu.Unlock()
	if dim == nil {
		dim = w.worldState.Dimension()
	}

	// if empty just reset and dont save anything
	worldState := w.worldState
	w.worldState = nil

	if len(worldState.StoredChunks) > 0 {
		// save image of the map
		if w.settings.SaveImage {
			f, _ := os.Create(worldState.Folder + ".png")
			png.Encode(f, w.mapUI.ToImage())
			f.Close()
		}

		// reset map, increase counter for
		w.serverState.worldCounter += 1
		w.mapUI.Reset()

		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			err := w.saveWorldState(worldState, w.session.Player, w.serverState.behaviorPack)
			if err != nil {
				w.log.Error(err)
			}
		}()
	}

	if !end {
		worldState, err := worldstate.New(w.ctx, w.serverState.dimensions, w.mapUI.SetChunk)
		if err != nil {
			w.log.Error(err)
		}
		worldState.VoidGen = w.settings.VoidGen
		worldState.SetDimension(dim)
		w.worldState = worldState
		w.openWorldState()
	}
}

func (w *worldsHandler) saveWorldState(worldState *worldstate.World, player proxy.Player, behaviorPack *behaviourpack.Pack) error {
	text := locale.Loc("saving_world", locale.Strmap{"Name": worldState.Name, "Count": len(worldState.StoredChunks)})
	w.log.Info(text)
	w.session.SendMessage(text)

	messages.SendEvent(&messages.EventProcessingWorldUpdate{
		WorldName: worldState.Name,
		State:     "Saving",
	})

	var playerSkins = make(map[uuid.UUID]*protocol.Skin)
	maps.Copy(playerSkins, w.serverState.playerSkins)

	err := worldState.Save(
		player, w.playerData(),
		w.serverState.behaviorPack,
		w.settings.ExcludedMobs,
		w.settings.Players, playerSkins,
		w.session.Server.GameData(), w.serverState.serverName,
		w.settings.EntityCulling,
		w.serverState.getEntityRenderDistance(),
	)
	if err != nil {
		return err
	}

	messages.SendEvent(&messages.EventProcessingWorldUpdate{
		WorldName: worldState.Name,
		State:     "Writing mcworld file",
	})

	filename := worldState.Folder + ".mcworld"
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
	total, active := worldState.EntityCounts(w.serverState.getEntityRenderDistance())
	w.log.WithFields(logrus.Fields{
		"Entities": fmt.Sprintf("%d (%d)", total, active),
		"Chunks":   len(worldState.StoredChunks),
	}).Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
	messages.SendEvent(&messages.EventFinishedSavingWorld{
		WorldName: worldState.Name,
		Filepath:  filename,
		Chunks:    len(worldState.StoredChunks),
		Entities:  total,
	})
	return nil
}

func (w *worldsHandler) defaultWorldName() string {
	worldName := "world"
	if w.serverState.worldCounter > 0 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	} else if w.serverState.worldName != "" {
		worldName = w.serverState.worldName
	}
	return worldName
}

func (w *worldsHandler) openWorldState() {
	worldName := utils.MakeValidFilename(w.defaultWorldName())
	serverName := utils.MakeValidFilename(w.serverState.serverName)
	folder := utils.PathData("worlds", serverName, worldName)

	w.worldState.BiomeRegistry = w.serverState.biomes
	w.worldState.BlockRegistry = w.serverState.blocks
	w.worldState.ResourcePacks = w.session.Server.ResourcePacks()
	w.worldState.UseHashedRids = w.serverState.useHashedRids
	w.worldState.Open(w.defaultWorldName(), folder)
}

func (w *worldsHandler) renameWorldState(name string) error {
	worldName := utils.MakeValidFilename(name)
	serverName := utils.MakeValidFilename(w.serverState.serverName)
	folder := utils.PathData("worlds", serverName, worldName)

	var err error
	w.currentWorld(func(world *worldstate.World) {
		err = world.Rename(name, folder)
	})
	return err
}
