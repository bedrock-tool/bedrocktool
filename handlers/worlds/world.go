package worlds

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"

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
	chunks             map[protocol.ChunkPos]*chunk.Chunk
	blockNBT           map[protocol.SubChunkPos][]map[string]any
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
	proxy *utils.ProxyContext
	mapUI *MapUI
	gui   utils.UI
	bp    *behaviourpack.BehaviourPack

	worldState  worldState
	serverState serverState
	settings    WorldSettings
}

var black16x16 = image.NewRGBA(image.Rect(0, 0, 16, 16))

func init() {
	for i := 3; i < len(black16x16.Pix); i += 4 {
		black16x16.Pix[i] = 255
	}
}

func NewWorldsHandler(ctx context.Context, ui utils.UI, settings WorldSettings) *utils.ProxyHandler {
	w := &worldsHandler{
		ctx:   ctx,
		mapUI: nil,
		gui:   ui,
		bp:    nil,

		serverState: serverState{
			ispre118:     false,
			worldCounter: 0,
			ChunkRadius:  0,

			playerInventory: nil,
			PlayerPos:       TPlayerPos{},
		},

		settings: settings,
	}
	w.mapUI = NewMapUI(w)
	w.Reset(w.CurrentName())

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
		OnClientConnect: func(conn *minecraft.Conn) {
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

func (w *worldsHandler) CurrentName() string {
	worldName := "world"
	if w.serverState.worldCounter > 1 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	}
	return worldName
}

func (w *worldsHandler) Reset(newName string) {
	w.worldState = worldState{
		dimension:          w.worldState.dimension,
		chunks:             make(map[protocol.ChunkPos]*chunk.Chunk),
		blockNBT:           make(map[protocol.SubChunkPos][]map[string]any),
		entities:           make(map[uint64]*entityState),
		openItemContainers: make(map[byte]*itemContainer),
		Name:               newName,
	}
	w.mapUI.Reset()
}

func (w *worldState) cullChunks() {
	keys := make([]protocol.ChunkPos, 0, len(w.chunks))
	for cp := range w.chunks {
		keys = append(keys, cp)
	}
	for _, cp := range fp.Filter(func(cp protocol.ChunkPos) bool {
		return !fp.Some(func(sc *chunk.SubChunk) bool {
			return !sc.Empty()
		})(w.chunks[cp].Sub())
	})(keys) {
		delete(w.chunks, cp)
	}
}

func (w *worldState) Save(folder string) (*mcdb.Provider, error) {
	provider, err := mcdb.New(logrus.StandardLogger(), folder, opt.DefaultCompression)
	if err != nil {
		return nil, err
	}

	// save chunk data
	for cp, c := range w.chunks {
		provider.SaveChunk((world.ChunkPos)(cp), c, w.dimension)
	}

	// save block nbt data
	blockNBT := make(map[world.ChunkPos][]map[string]any)
	for scp, v := range w.blockNBT { // 3d to 2d
		cp := world.ChunkPos{scp.X(), scp.Z()}
		blockNBT[cp] = append(blockNBT[cp], v...)
	}
	for cp, v := range blockNBT {
		err = provider.SaveBlockNBT(cp, v, w.dimension)
		if err != nil {
			logrus.Error(err)
		}
	}

	// save entities
	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.entities {
		cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
		chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity())
	}
	for cp, v := range chunkEntities {
		err = provider.SaveEntities(cp, v, w.dimension)
		if err != nil {
			logrus.Error(err)
		}
	}

	return provider, err
}

// SaveAndReset writes the world to a folder, resets all the chunks
func (w *worldsHandler) SaveAndReset() {
	w.worldState.cullChunks()
	if len(w.worldState.chunks) == 0 {
		w.Reset(w.CurrentName())
		return
	}

	logrus.Infof(locale.Loc("saving_world", locale.Strmap{"Name": w.worldState.Name, "Count": len(w.worldState.chunks)}))
	w.gui.Message(messages.SavingWorld{
		Name:   w.worldState.Name,
		Chunks: len(w.worldState.chunks),
	})

	// open world
	folder := path.Join("worlds", fmt.Sprintf("%s/%s", w.serverState.Name, w.worldState.Name))
	os.RemoveAll(folder)
	os.MkdirAll(folder, 0o777)
	provider, err := w.worldState.Save(folder)
	if err != nil {
		logrus.Error(err)
		return
	}

	err = provider.SaveLocalPlayerData(w.playerData())
	if err != nil {
		logrus.Error(err)
	}

	playerPos := w.proxy.Server.GameData().PlayerPosition
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	// write metadata
	s := provider.Settings()
	s.Spawn = spawnPos
	s.Name = w.worldState.Name

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

	if w.bp.HasContent() {
		if ld.Experiments == nil {
			ld.Experiments = map[string]any{}
		}
		ld.Experiments["data_driven_items"] = true
		ld.Experiments["experiments_ever_used"] = true
		ld.Experiments["saved_with_toggled_experiments"] = true
	}
	ld.RandomTickSpeed = 0
	s.CurrentTick = 0
	provider.SaveSettings(s)
	if err = provider.Close(); err != nil {
		logrus.Error(err)
	}

	w.serverState.worldCounter += 1

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
			var rdeps []dep
			for k, p := range packs {
				if p.Encrypted() && !p.CanDecrypt() {
					logrus.Warnf("Cant add %s, it is encrypted", p.Name())
					continue
				}
				logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": k}))
				name := p.Name()
				name = strings.ReplaceAll(name, ":", "_")
				packFolder := path.Join(folder, "resource_packs", name)
				os.MkdirAll(packFolder, 0o755)
				utils.UnpackZip(p, int64(p.Len()), packFolder)

				rdeps = append(rdeps, dep{
					PackID:  p.Manifest().Header.UUID,
					Version: p.Manifest().Header.Version,
				})
			}
			if len(rdeps) > 0 {
				addPacksJSON("world_resource_packs.json", rdeps)
			}
		}
	}

	if w.settings.SaveImage {
		f, _ := os.Create(folder + ".png")
		png.Encode(f, w.mapUI.ToImage())
		f.Close()
	}

	// zip it
	filename := folder + ".mcworld"
	if err := utils.ZipFolder(filename, folder); err != nil {
		logrus.Error(err)
	}
	logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
	//os.RemoveAll(folder)
	w.Reset(w.CurrentName())
	w.gui.Message(messages.SetUIState(messages.UIStateMain))
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
