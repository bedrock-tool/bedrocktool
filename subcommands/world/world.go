package world

import (
	"context"
	"encoding/json"
	"flag"
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

type worldSettings struct {
	// settings
	voidGen      bool
	withPacks    bool
	saveImage    bool
	blockUpdates bool
}

type worldState struct {
	Dim                world.Dimension
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

type worldsServer struct {
	ctx   context.Context
	proxy *utils.ProxyContext
	mapUI *MapUI
	gui   utils.UI
	bp    *behaviourpack.BehaviourPack

	worldState  worldState
	serverState serverState
	settings    worldSettings
}

func NewWorldsServer(ctx context.Context, proxy *utils.ProxyContext, ServerName string, ui utils.UI) *worldsServer {
	w := &worldsServer{
		ctx:   ctx,
		proxy: proxy,
		mapUI: nil,
		gui:   ui,
		bp:    behaviourpack.New(ServerName),

		serverState: serverState{
			ispre118:     false,
			worldCounter: 0,
			ChunkRadius:  0,

			playerInventory: nil,
			PlayerPos:       TPlayerPos{},

			Name: ServerName,
		},

		settings: worldSettings{},
	}
	w.mapUI = NewMapUI(w)
	w.Reset(w.CurrentName())

	w.gui.Message(messages.Init{
		Handler: nil,
	})

	return w
}

var dimensionIDMap = map[uint8]world.Dimension{
	0: world.Overworld,
	1: world.Nether,
	2: world.End,
	// < 1.18
	10: world.Overworld_legacy,
	11: world.Nether,
	12: world.End,
}

var (
	black16x16 = image.NewRGBA(image.Rect(0, 0, 16, 16))
)

func init() {
	for i := 3; i < len(black16x16.Pix); i += 4 {
		black16x16.Pix[i] = 255
	}
	utils.RegisterCommand(&WorldCMD{})
}

type WorldCMD struct {
	ServerAddress string
	Packs         bool
	EnableVoid    bool
	SaveImage     bool
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return locale.Loc("world_synopsis", nil) }

func (c *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.BoolVar(&c.Packs, "packs", false, locale.Loc("save_packs_with_world", nil))
	f.BoolVar(&c.EnableVoid, "void", true, locale.Loc("enable_void", nil))
	f.BoolVar(&c.SaveImage, "image", false, locale.Loc("save_image", nil))
}

func (c *WorldCMD) Execute(ctx context.Context, ui utils.UI) error {
	serverAddress, hostname, err := ui.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}

	w := NewWorldsServer(ctx, proxy, hostname, ui)
	w.settings = worldSettings{
		voidGen:   c.EnableVoid,
		withPacks: c.Packs,
		saveImage: c.SaveImage,
	}

	proxy.AlwaysGetPacks = true
	proxy.GameDataModifier = func(gd *minecraft.GameData) {
		gd.ClientSideGeneration = false
	}
	proxy.ConnectCB = w.OnConnect
	proxy.OnClientConnect = func(hasClient bool) {
		w.gui.Message(messages.SetUIState(messages.UIStateConnecting))
	}
	proxy.PacketCB = func(pk packet.Packet, toServer bool, _ time.Time) (packet.Packet, error) {
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
	}

	w.gui.Message(messages.SetUIState(messages.UIStateConnect))
	err = w.proxy.Run(ctx, serverAddress)
	if err != nil {
		return err
	}
	w.SaveAndReset()
	ui.Message(messages.SetUIState(messages.UIStateFinished))
	return nil
}

func (w *worldsServer) SetPlayerPos(Position mgl32.Vec3, Pitch, Yaw, HeadYaw float32) {
	last := w.serverState.PlayerPos
	current := TPlayerPos{
		Position: Position,
		Pitch:    Pitch,
		Yaw:      Yaw,
		HeadYaw:  HeadYaw,
	}
	w.serverState.PlayerPos = current

	if int(last.Position.X()) != int(current.Position.X()) || int(last.Position.Z()) != int(current.Position.Z()) {
		w.mapUI.SchedRedraw()
	}
}

func (w *worldsServer) setVoidGen(val bool, fromUI bool) bool {
	w.settings.voidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.settings.voidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.proxy.SendMessage(s)

	if !fromUI {
		w.gui.Message(messages.SetVoidGen{
			Value: w.settings.voidGen,
		})
	}

	return true
}

func (w *worldsServer) setWorldName(val string, fromUI bool) bool {
	w.worldState.Name = val
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.worldState.Name}))

	if !fromUI {
		w.gui.Message(messages.SetWorldName{
			WorldName: w.worldState.Name,
		})
	}

	return true
}

func (w *worldsServer) CurrentName() string {
	worldName := "world"
	if w.serverState.worldCounter > 1 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	}
	return worldName
}

func (w *worldsServer) Reset(newName string) {
	w.worldState = worldState{
		Dim:                nil,
		chunks:             make(map[protocol.ChunkPos]*chunk.Chunk),
		blockNBT:           make(map[protocol.SubChunkPos][]map[string]any),
		entities:           make(map[uint64]*entityState),
		openItemContainers: make(map[byte]*itemContainer),
		Name:               newName,
	}
	w.mapUI.Reset()
}

// SaveAndReset writes the world to a folder, resets all the chunks
func (w *worldsServer) SaveAndReset() {

	// cull empty chunks
	keys := make([]protocol.ChunkPos, 0, len(w.worldState.chunks))
	for cp := range w.worldState.chunks {
		keys = append(keys, cp)
	}

	for _, cp := range fp.Filter(func(cp protocol.ChunkPos) bool {
		return fp.Some(func(sc *chunk.SubChunk) bool {
			return !sc.Empty()
		})(w.worldState.chunks[cp].Sub())
	})(keys) {
		delete(w.worldState.chunks, cp)
	}

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

	provider, err := mcdb.New(logrus.StandardLogger(), folder, opt.DefaultCompression)
	if err != nil {
		logrus.Fatal(err)
	}

	// save chunk data
	for cp, c := range w.worldState.chunks {
		provider.SaveChunk((world.ChunkPos)(cp), c, w.worldState.Dim)
	}

	// save block nbt data
	blockNBT := make(map[world.ChunkPos][]map[string]any)
	for scp, v := range w.worldState.blockNBT { // 3d to 2d
		cp := world.ChunkPos{scp.X(), scp.Z()}
		blockNBT[cp] = append(blockNBT[cp], v...)
	}
	for cp, v := range blockNBT {
		err = provider.SaveBlockNBT(cp, v, w.worldState.Dim)
		if err != nil {
			logrus.Error(err)
		}
	}

	// save entities
	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.worldState.entities {
		cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
		chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity())
	}

	for cp, v := range chunkEntities {
		err = provider.SaveEntities(cp, v, w.worldState.Dim)
		if err != nil {
			logrus.Error(err)
		}
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

	ld.RandomSeed = int64(gd.WorldSeed)

	// void world
	if w.settings.voidGen {
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
		w.settings.withPacks = true
	}

	// add resource packs
	if w.settings.withPacks {
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
				packFolder := path.Join(folder, "resource_packs", p.Name())
				os.MkdirAll(packFolder, 0o755)
				utils.UnpackZip(p, int64(p.Len()), packFolder)

				rdeps = append(rdeps, dep{
					PackID:  p.Manifest().Header.UUID,
					Version: p.Manifest().Header.Version,
				})
			}
			_ = rdeps
			/*
				if len(rdeps) > 0 {
					addPacksJSON("world_resource_packs.json", rdeps)
				}
			*/
		}
	}

	if w.settings.saveImage {
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

func (w *worldsServer) OnConnect(err error) bool {
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
			dimensionID += 10
		}
		w.worldState.Dim = dimensionIDMap[uint8(dimensionID)]
	}

	w.proxy.SendMessage(locale.Loc("use_setname", nil))

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
			return w.setVoidGen(!w.settings.voidGen, false)
		},
		Cmd: protocol.Command{
			Name:        "void",
			Description: locale.Loc("void_desc", nil),
		},
	})

	w.mapUI.Start()
	return true
}
