package worlds

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/flytam/filenamify"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/repeale/fp-go"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type TPlayerPos struct {
	Position mgl32.Vec3
	Pitch    float32
	Yaw      float32
	HeadYaw  float32
}

// the state used for drawing and saving

type WorldSettings struct {
	// settings
	VoidGen         bool
	WithPacks       bool
	SaveImage       bool
	SaveEntities    bool
	SaveInventories bool
	BlockUpdates    bool
}

type worldState struct {
	dimension          world.Dimension
	chunks             map[world.ChunkPos]*chunk.Chunk
	blockNBTs          map[cube.Pos]map[string]any
	entities           map[uint64]*entityState
	openItemContainers map[byte]*itemContainer
	Name               string
}

type serverState struct {
	ChunkRadius  int
	ispre118     bool
	worldCounter int

	playerInventory []protocol.ItemInstance
	PlayerPos       TPlayerPos

	Name string
}

type worldsHandler struct {
	ctx   context.Context
	wg    sync.WaitGroup
	proxy *utils.ProxyContext
	mapUI *MapUI
	gui   utils.UI
	bp    *behaviourpack.BehaviourPack

	worldState  worldState
	serverState serverState
	settings    WorldSettings
}

func NewWorldsHandler(ctx context.Context, ui utils.UI, settings WorldSettings) *utils.ProxyHandler {
	w := &worldsHandler{
		ctx: ctx,
		gui: ui,

		serverState: serverState{
			ispre118:     false,
			worldCounter: 0,
			ChunkRadius:  0,
			PlayerPos:    TPlayerPos{},
		},

		settings: settings,
	}
	w.mapUI = NewMapUI(w)
	w.Reset()

	return &utils.ProxyHandler{
		Name: "Worlds",
		ProxyRef: func(pc *utils.ProxyContext) {
			w.proxy = pc

			w.proxy.AddCommand(utils.IngameCommand{
				Exec: func(cmdline []string) bool {
					return w.setWorldName(strings.Join(cmdline, " "), false)
				},
				Cmd: protocol.Command{
					Name:        "setname",
					Description: locale.Loc("setname_desc", nil),
					Overloads: []protocol.CommandOverload{{
						Parameters: []protocol.CommandParameter{{
							Name:     "name",
							Type:     protocol.CommandArgTypeString,
							Optional: false,
						}},
					}},
				},
			})

			w.proxy.AddCommand(utils.IngameCommand{
				Exec: func(cmdline []string) bool {
					return w.setVoidGen(!w.settings.VoidGen, false)
				},
				Cmd: protocol.Command{
					Name:        "void",
					Description: locale.Loc("void_desc", nil),
				},
			})
		},
		AddressAndName: func(address, hostname string) error {
			w.bp = behaviourpack.New(hostname)
			w.serverState.Name = hostname
			return nil
		},
		OnClientConnect: func(conn minecraft.IConn) {
			w.gui.Message(messages.SetUIState(messages.UIStateConnecting))
		},
		GameDataModifier: func(gd *minecraft.GameData) {
			gd.ClientSideGeneration = false
		},
		ConnectCB: w.OnConnect,
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time) (packet.Packet, error) {
			forward := true

			if toServer {
				// from client
				pk = w.processItemPacketsClient(pk, &forward)
				pk = w.processMapPacketsClient(pk, &forward)
			} else {
				// from server
				switch pk := pk.(type) {
				case *packet.ChunkRadiusUpdated:
					w.serverState.ChunkRadius = int(pk.ChunkRadius)
					pk.ChunkRadius = 80
				}
				pk = w.processItemPacketsServer(pk)
				pk = w.ProcessChunkPackets(pk)
				pk = w.ProcessEntityPackets(pk)
			}

			if !forward {
				return nil, nil
			}
			return pk, nil
		},
		OnEnd: func() {
			w.SaveAndReset()
			w.wg.Wait()
		},
	}
}

func (w *worldsHandler) setVoidGen(val bool, fromUI bool) bool {
	w.settings.VoidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.settings.VoidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.proxy.SendMessage(s)

	if !fromUI {
		w.gui.Message(messages.SetVoidGen{
			Value: w.settings.VoidGen,
		})
	}

	return true
}

func (w *worldsHandler) setWorldName(val string, fromUI bool) bool {
	w.worldState.Name = val
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.worldState.Name}))

	if !fromUI {
		w.gui.Message(messages.SetWorldName{
			WorldName: w.worldState.Name,
		})
	}

	return true
}

func (w *worldsHandler) currentName() string {
	worldName := "world"
	if w.serverState.worldCounter > 0 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	}
	return worldName
}

func (w *worldsHandler) Reset() {
	w.worldState = worldState{
		dimension:          w.worldState.dimension,
		chunks:             make(map[world.ChunkPos]*chunk.Chunk),
		blockNBTs:          make(map[cube.Pos]map[string]any),
		entities:           make(map[uint64]*entityState),
		openItemContainers: make(map[byte]*itemContainer),
		Name:               w.currentName(),
	}
	w.mapUI.Reset()
}

func (w *worldState) cullChunks() {
	keys := make([]world.ChunkPos, 0, len(w.chunks))
	for cp := range w.chunks {
		keys = append(keys, cp)
	}
	for _, cp := range fp.Filter(func(cp world.ChunkPos) bool {
		return !fp.Some(func(sc *chunk.SubChunk) bool {
			return !sc.Empty()
		})(w.chunks[cp].Sub())
	})(keys) {
		delete(w.chunks, cp)
	}
}

type dummyBlock struct {
	id  string
	nbt map[string]any
}

func (d *dummyBlock) EncodeBlock() (string, map[string]any) {
	return d.id, d.nbt
}

func (d *dummyBlock) Hash() uint64 {
	return 0
}

func (d *dummyBlock) Model() world.BlockModel {
	return nil
}

func (d *dummyBlock) Color() color.RGBA {
	return color.RGBA{0, 0, 0, 0}
}

func (d *dummyBlock) DecodeNBT(data map[string]any) any {
	return nil
}

func (d *dummyBlock) EncodeNBT() map[string]any {
	return d.nbt
}

func (w *worldState) Save(folder string) (*mcdb.DB, error) {
	provider, err := mcdb.Config{
		Log:         logrus.StandardLogger(),
		Compression: opt.DefaultCompression,
	}.Open(folder)
	if err != nil {
		return nil, err
	}

	chunkBlockNBT := make(map[world.ChunkPos]map[cube.Pos]world.Block)
	for bp, blockNBT := range w.blockNBTs { // 3d to 2d
		cp := world.ChunkPos{int32(bp.X()) >> 4, int32(bp.Z()) >> 4}
		m, ok := chunkBlockNBT[cp]
		if !ok {
			m = make(map[cube.Pos]world.Block)
			chunkBlockNBT[cp] = m
		}
		id := blockNBT["id"].(string)
		m[bp] = &dummyBlock{id, blockNBT}
	}

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.entities {
		cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
		chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity())
	}

	// save chunk data
	for cp, c := range w.chunks {
		column := &world.Column{
			Chunk:         c,
			BlockEntities: chunkBlockNBT[cp],
			Entities:      chunkEntities[cp],
		}
		err = provider.StoreColumn(cp, w.dimension, column)
		if err != nil {
			logrus.Error(err)
		}
	}

	return provider, err
}

func (w *worldsHandler) SaveAndReset() {
	w.worldState.cullChunks()
	if len(w.worldState.chunks) == 0 {
		w.Reset()
		return
	}

	worldStateCopy := w.worldState
	playerData := w.playerData()
	playerPos := w.serverState.PlayerPos.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	var img image.Image
	if w.settings.SaveImage {
		img = w.mapUI.ToImage()
	}

	w.serverState.worldCounter += 1
	w.Reset()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		logrus.Infof(locale.Loc("saving_world", locale.Strmap{"Name": worldStateCopy.Name, "Count": len(worldStateCopy.chunks)}))
		w.gui.Message(messages.SavingWorld{
			Name:   w.worldState.Name,
			Chunks: len(w.worldState.chunks),
		})

		// open world
		folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, worldStateCopy.Name)
		os.RemoveAll(folder)
		os.MkdirAll(folder, 0o777)
		provider, err := worldStateCopy.Save(folder)
		if err != nil {
			logrus.Error(err)
			return
		}

		err = provider.SaveLocalPlayerData(playerData)
		if err != nil {
			logrus.Error(err)
			return
		}

		// write metadata
		s := provider.Settings()
		s.Spawn = spawnPos
		s.Name = worldStateCopy.Name

		// set gamerules
		ld := provider.LevelDat()
		gd := w.proxy.Server.GameData()
		ld.RandomSeed = int64(gd.WorldSeed)
		for _, gr := range gd.GameRules {
			switch gr.Name {
			case "commandblockoutput":
				ld.CommandBlockOutput = gr.Value.(bool)
			case "maxcommandchainlength":
				ld.MaxCommandChainLength = int32(gr.Value.(uint32))
			case "commandblocksenabled":
				ld.CommandsEnabled = gr.Value.(bool)
			case "dodaylightcycle":
				ld.DoDayLightCycle = gr.Value.(bool)
			case "doentitydrops":
				ld.DoEntityDrops = gr.Value.(bool)
			case "dofiretick":
				ld.DoFireTick = gr.Value.(bool)
			case "domobloot":
				ld.DoMobLoot = gr.Value.(bool)
			case "domobspawning":
				ld.DoMobSpawning = gr.Value.(bool)
			case "dotiledrops":
				ld.DoTileDrops = gr.Value.(bool)
			case "doweathercycle":
				ld.DoWeatherCycle = gr.Value.(bool)
			case "drowningdamage":
				ld.DrowningDamage = gr.Value.(bool)
			case "doinsomnia":
				ld.DoInsomnia = gr.Value.(bool)
			case "falldamage":
				ld.FallDamage = gr.Value.(bool)
			case "firedamage":
				ld.FireDamage = gr.Value.(bool)
			case "keepinventory":
				ld.KeepInventory = gr.Value.(bool)
			case "mobgriefing":
				ld.MobGriefing = gr.Value.(bool)
			case "pvp":
				ld.PVP = gr.Value.(bool)
			case "showcoordinates":
				ld.ShowCoordinates = gr.Value.(bool)
			case "naturalregeneration":
				ld.NaturalRegeneration = gr.Value.(bool)
			case "tntexplodes":
				ld.TNTExplodes = gr.Value.(bool)
			case "sendcommandfeedback":
				ld.SendCommandFeedback = gr.Value.(bool)
			case "randomtickspeed":
				ld.RandomTickSpeed = int32(gr.Value.(uint32))
			case "doimmediaterespawn":
				ld.DoImmediateRespawn = gr.Value.(bool)
			case "showdeathmessages":
				ld.ShowDeathMessages = gr.Value.(bool)
			case "functioncommandlimit":
				ld.FunctionCommandLimit = int32(gr.Value.(uint32))
			case "spawnradius":
				ld.SpawnRadius = int32(gr.Value.(uint32))
			case "showtags":
				ld.ShowTags = gr.Value.(bool)
			case "freezedamage":
				ld.FreezeDamage = gr.Value.(bool)
			case "respawnblocksexplode":
				ld.RespawnBlocksExplode = gr.Value.(bool)
			case "showbordereffect":
				ld.ShowBorderEffect = gr.Value.(bool)
			// todo
			default:
				logrus.Warnf(locale.Loc("unknown_gamerule", locale.Strmap{"Name": gr.Name}))
			}
		}

		// void world
		if w.settings.VoidGen {
			ld.FlatWorldLayers = `{"biome_id":1,"block_layers":[{"block_data":0,"block_id":0,"count":1},{"block_data":0,"block_id":0,"count":2},{"block_data":0,"block_id":0,"count":1}],"encoding_version":3,"structure_options":null}`
			ld.Generator = 2
		}

		ld.RandomTickSpeed = 0
		s.CurrentTick = 0

		if w.bp.HasContent() {
			if ld.Experiments == nil {
				ld.Experiments = map[string]any{}
			}
			ld.Experiments["data_driven_items"] = true
			ld.Experiments["experiments_ever_used"] = true
			ld.Experiments["saved_with_toggled_experiments"] = true
		}

		provider.SaveSettings(s)
		err = provider.Close()
		if err != nil {
			logrus.Error(err)
			return
		}

		if w.settings.SaveImage {
			f, _ := os.Create(folder + ".png")
			png.Encode(f, img)
			f.Close()
		}

		w.AddPacks(folder)

		// zip it
		filename := folder + ".mcworld"
		err = utils.ZipFolder(filename, folder)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
		//os.RemoveAll(folder)
		w.gui.Message(messages.SetUIState(messages.UIStateMain))
	}()
}

func (w *worldsHandler) AddPacks(folder string) {
	type dep struct {
		PackID  string `json:"pack_id"`
		Version [3]int `json:"version"`
	}
	addPacksJSON := func(name string, deps []dep) {
		f, err := os.Create(path.Join(folder, name))
		if err != nil {
			logrus.Error(err)
			return
		}
		defer f.Close()
		if err := json.NewEncoder(f).Encode(deps); err != nil {
			logrus.Error(err)
			return
		}
	}

	// save behaviourpack
	if w.bp.HasContent() {
		name := strings.ReplaceAll(w.serverState.Name, "./", "")
		name = strings.ReplaceAll(name, "/", "-")
		name = strings.ReplaceAll(name, ":", "_")
		packFolder := path.Join(folder, "behavior_packs", name)
		os.MkdirAll(packFolder, 0o755)

		for _, p := range w.proxy.Server.ResourcePacks() {
			p := utils.PackFromBase(p)
			w.bp.CheckAddLink(p)
		}

		w.bp.Save(packFolder)
		addPacksJSON("world_behavior_packs.json", []dep{{
			PackID:  w.bp.Manifest.Header.UUID,
			Version: w.bp.Manifest.Header.Version,
		}})

		// force resource packs for worlds with custom blocks
		w.settings.WithPacks = true
	}

	// add resource packs
	if w.settings.WithPacks {
		packs, err := utils.GetPacks(w.proxy.Server)
		if err != nil {
			logrus.Error(err)
		} else {
			packNames := make(map[string]int)
			for _, pack := range packs {
				packNames[pack.Name()] += 1
			}

			var rdeps []dep
			for _, pack := range packs {
				if pack.Encrypted() && !pack.CanDecrypt() {
					logrus.Warnf("Cant add %s, it is encrypted", pack.Name())
					continue
				}
				logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": pack.Name()}))

				packName := pack.Name()
				if packNames[packName] > 1 {
					packName += "_" + pack.UUID()
				}
				packName, _ = filenamify.FilenamifyV2(packName)
				packFolder := path.Join(folder, "resource_packs", packName)
				os.MkdirAll(packFolder, 0o755)
				utils.UnpackZip(pack, int64(pack.Len()), packFolder)

				rdeps = append(rdeps, dep{
					PackID:  pack.Manifest().Header.UUID,
					Version: pack.Manifest().Header.Version,
				})
			}
			if len(rdeps) > 0 {
				addPacksJSON("world_resource_packs.json", rdeps)
			}
		}
	}
}

func (w *worldsHandler) OnConnect(err error) bool {
	w.gui.Message(messages.SetUIState(messages.UIStateMain))
	if err != nil {
		return false
	}
	gd := w.proxy.Server.GameData()
	w.serverState.ChunkRadius = int(gd.ChunkRadius)
	w.proxy.ClientWritePacket(&packet.ChunkRadiusUpdated{
		ChunkRadius: 80,
	})

	world.InsertCustomItems(gd.Items)
	for _, ie := range gd.Items {
		w.bp.AddItem(ie)
	}

	mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
	MapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
	if gd.ServerAuthoritativeInventory {
		MapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
	}

	if len(gd.CustomBlocks) > 0 {
		logrus.Info(locale.Loc("using_customblocks", nil))
		for _, be := range gd.CustomBlocks {
			w.bp.AddBlock(be)
		}
		// telling the chunk code what custom blocks there are so it can generate offsets
		world.InsertCustomBlocks(gd.CustomBlocks)
	}

	{ // check game version
		gv := strings.Split(gd.BaseGameVersion, ".")
		var err error
		if len(gv) > 1 {
			var ver int
			ver, err = strconv.Atoi(gv[1])
			w.serverState.ispre118 = ver < 18
		}
		if err != nil || len(gv) <= 1 {
			logrus.Info(locale.Loc("guessing_version", nil))
		}

		dimensionID := gd.Dimension
		if w.serverState.ispre118 {
			logrus.Info(locale.Loc("using_under_118", nil))
			if dimensionID == 0 {
				dimensionID += 10
			}
		}
		w.worldState.dimension, _ = world.DimensionByID(int(dimensionID))
	}

	w.proxy.SendMessage(locale.Loc("use_setname", nil))
	w.mapUI.Start()
	return true
}
