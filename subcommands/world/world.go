package world

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	//_ "github.com/df-mc/dragonfly/server/block" // to load blocks
	//_ "net/http/pprof"
)

type TPlayerPos struct {
	Position mgl32.Vec3
	Pitch    float32
	Yaw      float32
	HeadYaw  float32
}

type itemContainer struct {
	OpenPacket *packet.ContainerOpen
	Content    *packet.InventoryContent
}

// the state used for drawing and saving

type WorldState struct {
	ctx                context.Context
	ispre118           bool
	voidgen            bool
	chunks             map[protocol.ChunkPos]*chunk.Chunk
	blockNBT           map[protocol.SubChunkPos][]map[string]any
	openItemContainers map[byte]*itemContainer

	Dim          world.Dimension
	WorldName    string
	ServerName   string
	worldCounter int
	packs        map[string]*resource.Pack
	bp           *behaviourpack.BehaviourPack

	withPacks           bool
	saveImage           bool
	experimentInventory bool

	PlayerPos TPlayerPos
	proxy     *utils.ProxyContext

	// ui
	ui *MapUI
}

func NewWorldState() *WorldState {
	w := &WorldState{
		chunks:             make(map[protocol.ChunkPos]*chunk.Chunk),
		blockNBT:           make(map[protocol.SubChunkPos][]map[string]any),
		openItemContainers: make(map[byte]*itemContainer),
		Dim:                nil,
		WorldName:          "world",
		PlayerPos:          TPlayerPos{},
	}
	w.ui = NewMapUI(w)
	return w
}

var dimension_ids = map[uint8]world.Dimension{
	0: world.Overworld,
	1: world.Nether,
	2: world.End,
	// < 1.18
	10: world.Overworld_legacy,
	11: world.Nether,
	12: world.End,
}

var (
	black_16x16  = image.NewRGBA(image.Rect(0, 0, 16, 16))
	Offset_table [24]protocol.SubChunkOffset
)

func init() {
	for i := range Offset_table {
		Offset_table[i] = protocol.SubChunkOffset{0, int8(i), 0}
	}
	draw.Draw(black_16x16, image.Rect(0, 0, 16, 16), image.Black, image.Point{}, draw.Src)
	utils.RegisterCommand(&WorldCMD{})
}

type WorldCMD struct {
	Address             string
	packs               bool
	enableVoid          bool
	saveImage           bool
	experimentInventory bool
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return locale.Loc("world_synopsis", nil) }

func (p *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.Address, "address", "", locale.Loc("remote_address", nil))
	f.BoolVar(&p.packs, "packs", false, locale.Loc("save_packs_with_world", nil))
	f.BoolVar(&p.enableVoid, "void", true, locale.Loc("enable_void", nil))
	f.BoolVar(&p.saveImage, "image", false, locale.Loc("save_image", nil))
	f.BoolVar(&p.experimentInventory, "inv", false, locale.Loc("test_block_inv", nil))
}

func (c *WorldCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *WorldCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	/*
		go func() {
			http.ListenAndServe(":8000", nil)
		}()
	*/

	server_address, hostname, err := utils.ServerInput(ctx, c.Address)
	if err != nil {
		logrus.Error(err)
		return 1
	}

	w := NewWorldState()
	w.voidgen = c.enableVoid
	w.ServerName = hostname
	w.withPacks = c.packs
	w.saveImage = c.saveImage
	w.experimentInventory = c.experimentInventory
	w.ctx = ctx

	proxy := utils.NewProxy()
	proxy.AlwaysGetPacks = true
	proxy.ConnectCB = w.OnConnect
	proxy.PacketCB = func(pk packet.Packet, proxy *utils.ProxyContext, toServer bool) (packet.Packet, error) {
		var forward bool
		if toServer {
			pk, forward = w.ProcessPacketClient(pk)
		} else {
			pk, forward = w.ProcessPacketServer(pk)
		}
		if !forward {
			return nil, nil
		}
		return pk, nil
	}

	super_verbose_log := false
	if super_verbose_log {
		utils.F_Log, err = os.Create("packets.log")
		if err != nil {
			logrus.Error(err)
		}
	}

	err = proxy.Run(ctx, server_address)
	if err != nil {
		logrus.Error(err)
	} else {
		w.SaveAndReset()
	}
	return 0
}

func (w *WorldState) setnameCommand(cmdline []string) bool {
	w.WorldName = strings.Join(cmdline, " ")
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.WorldName}))
	return true
}

func (w *WorldState) toggleVoid(cmdline []string) bool {
	w.voidgen = !w.voidgen
	var s string
	if w.voidgen {
		s = locale.Loc("void_generator_true", nil)
	} else {
		s = locale.Loc("void_generator_false", nil)
	}
	w.proxy.SendMessage(s)
	return true
}

func (w *WorldState) ProcessLevelChunk(pk *packet.LevelChunk) {
	_, exists := w.chunks[pk.Position]
	if exists {
		return
	}

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, int(pk.SubChunkCount), w.Dim.Range(), w.ispre118)
	if err != nil {
		logrus.Error(err)
		return
	}
	if blockNBTs != nil {
		w.blockNBT[protocol.SubChunkPos{
			pk.Position.X(), 0, pk.Position.Z(),
		}] = blockNBTs
	}

	w.chunks[pk.Position] = ch

	if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLegacy {
		w.ui.SetChunk(pk.Position, ch)
	} else {
		w.ui.SetChunk(pk.Position, nil)
		// request all the subchunks

		max := w.Dim.Range().Height() / 16
		if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLimited {
			max = int(pk.HighestSubChunk)
		}

		w.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(w.Dim.EncodeDimension()),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: Offset_table[:max],
		})
	}
}

func (w *WorldState) ProcessSubChunk(pk *packet.SubChunk) {
	pos_to_redraw := make(map[protocol.ChunkPos]bool)

	for _, sub := range pk.SubChunkEntries {
		var (
			abs_x  = pk.Position[0] + int32(sub.Offset[0])
			abs_y  = pk.Position[1] + int32(sub.Offset[1])
			abs_z  = pk.Position[2] + int32(sub.Offset[2])
			subpos = protocol.SubChunkPos{abs_x, abs_y, abs_z}
			pos    = protocol.ChunkPos{abs_x, abs_z}
		)
		ch := w.chunks[pos]
		if ch == nil {
			logrus.Error(locale.Loc("subchunk_before_chunk", nil))
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(abs_y), &sub)
		if err != nil {
			logrus.Error(err)
		}
		if blockNBT != nil {
			w.blockNBT[subpos] = blockNBT
		}

		pos_to_redraw[pos] = true
	}

	// redraw the chunks
	for pos := range pos_to_redraw {
		w.ui.SetChunk(pos, w.chunks[pos])
	}
	w.ui.SchedRedraw()
}

func (w *WorldState) ProcessAnimate(pk *packet.Animate) {
	if pk.ActionType == packet.AnimateActionSwingArm {
		w.ui.ChangeZoom()
		w.proxy.SendPopup(locale.Loc("zoom_level", locale.Strmap{"Level": w.ui.zoomLevel}))
	}
}

func (w *WorldState) ProcessChangeDimension(pk *packet.ChangeDimension) {
	if len(w.chunks) > 0 {
		w.SaveAndReset()
	} else {
		logrus.Info(locale.Loc("not_saving_empty", nil))
		w.Reset()
	}
	dim_id := pk.Dimension
	if w.ispre118 {
		dim_id += 10
	}
	w.Dim = dimension_ids[uint8(dim_id)]
}

func (w *WorldState) SetPlayerPos(Position mgl32.Vec3, Pitch, Yaw, HeadYaw float32) {
	last := w.PlayerPos
	w.PlayerPos = TPlayerPos{
		Position: Position,
		Pitch:    Pitch,
		Yaw:      Yaw,
		HeadYaw:  HeadYaw,
	}

	if int(last.Position.X()) != int(w.PlayerPos.Position.X()) || int(last.Position.Z()) != int(w.PlayerPos.Position.Z()) {
		w.ui.SchedRedraw()
	}
}

func (w *WorldState) Reset() {
	w.chunks = make(map[protocol.ChunkPos]*chunk.Chunk)
	w.WorldName = fmt.Sprintf("world-%d", w.worldCounter)
	w.ui.Reset()
}

// writes the world to a folder, resets all the chunks
func (w *WorldState) SaveAndReset() {
	if len(w.chunks) == 0 {
		w.Reset()
		return
	}
	logrus.Infof(locale.Loc("saving_world", locale.Strmap{"Name": w.WorldName, "Count": len(w.chunks)}))

	// open world
	folder := path.Join("worlds", fmt.Sprintf("%s/%s", w.ServerName, w.WorldName))
	os.RemoveAll(folder)
	os.MkdirAll(folder, 0o777)

	provider, err := mcdb.New(logrus.StandardLogger(), folder, opt.DefaultCompression)
	if err != nil {
		logrus.Fatal(err)
	}

	// save chunk data
	for cp, c := range w.chunks {
		provider.SaveChunk((world.ChunkPos)(cp), c, w.Dim)
	}

	// save block nbt data
	blockNBT := make(map[protocol.ChunkPos][]map[string]any)
	for scp, v := range w.blockNBT { // 3d to 2d
		cp := protocol.ChunkPos{scp.X(), scp.Z()}
		blockNBT[cp] = append(blockNBT[cp], v...)
	}
	for cp, v := range blockNBT {
		err = provider.SaveBlockNBT((world.ChunkPos)(cp), v, w.Dim)
		if err != nil {
			logrus.Error(err)
		}
	}

	// write metadata
	s := provider.Settings()
	player := w.proxy.Server.GameData().PlayerPosition
	s.Spawn = cube.Pos{
		int(player.X()),
		int(player.Y()),
		int(player.Z()),
	}
	s.Name = w.WorldName

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
	if w.voidgen {
		ld.FlatWorldLayers = `{"biome_id":1,"block_layers":[{"block_data":0,"block_id":0,"count":1},{"block_data":0,"block_id":0,"count":2},{"block_data":0,"block_id":0,"count":1}],"encoding_version":3,"structure_options":null}`
		ld.Generator = 2
	}

	provider.SaveSettings(s)
	provider.Close()
	w.worldCounter += 1

	type dep struct {
		PackId  string `json:"pack_id"`
		Version [3]int `json:"version"`
	}
	add_packs_json := func(name string, deps []dep) {
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

	{
		var rdeps []dep
		for k, p := range w.packs {
			logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": k}))
			pack_folder := path.Join(folder, "resource_packs", k)
			os.MkdirAll(pack_folder, 0o755)
			data := make([]byte, p.Len())
			p.ReadAt(data, 0)
			utils.UnpackZip(bytes.NewReader(data), int64(len(data)), pack_folder)
			rdeps = append(rdeps, dep{
				PackId:  p.Manifest().Header.Name,
				Version: p.Manifest().Header.Version,
			})
		}
		add_packs_json("world_resource_packs.json", rdeps)
	}

	if w.bp != nil {
		name := strings.ReplaceAll(w.ServerName, "/", "-") + "_blocks"
		pack_folder := path.Join(folder, "behavior_packs", name)
		os.MkdirAll(pack_folder, 0o755)

		for _, p := range w.proxy.Server.ResourcePacks() {
			p := utils.PackFromBase(p)
			w.bp.CheckAddLink(p)
		}

		w.bp.Save(pack_folder)
		add_packs_json("world_behavior_packs.json", []dep{{
			PackId:  w.bp.Manifest.Header.UUID,
			Version: w.bp.Manifest.Header.Version,
		}})

		if ld.Experiments == nil {
			ld.Experiments = map[string]any{}
		}
		ld.Experiments["data_driven_items"] = true
		ld.Experiments["experiments_ever_used"] = true
		ld.Experiments["saved_with_toggled_experiments"] = true
	}

	if w.saveImage {
		f, _ := os.Create(folder + ".png")
		png.Encode(f, w.ui.ToImage())
		f.Close()
	}

	// zip it
	filename := folder + ".mcworld"

	if err := utils.ZipFolder(filename, folder); err != nil {
		fmt.Println(err)
	}
	logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
	os.RemoveAll(folder)
	w.Reset()
}

func (w *WorldState) OnConnect(proxy *utils.ProxyContext) {
	w.proxy = proxy
	gd := w.proxy.Server.GameData()

	world.InsertCustomItems(gd.Items)

	map_item_id, _ := world.ItemRidByName("minecraft:filled_map")
	MAP_ITEM_PACKET.Content[0].Stack.ItemType.NetworkID = map_item_id
	if gd.ServerAuthoritativeInventory {
		MAP_ITEM_PACKET.Content[0].StackNetworkID = rand.Int31n(32)
	}

	if len(gd.CustomBlocks) > 0 {
		logrus.Info(locale.Loc("using_customblocks", nil))

		w.bp = behaviourpack.New(w.ServerName + " Custom Blocks")
		for _, be := range gd.CustomBlocks {
			w.bp.AddBlock(be)
		}

		// telling the chunk code what custom blocks there are so it can generate offsets
		world.InsertCustomBlocks(gd.CustomBlocks)
	}

	if w.withPacks {
		go func() {
			w.packs, _ = utils.GetPacks(w.proxy.Server)
		}()
	}

	{ // check game version
		gv := strings.Split(gd.BaseGameVersion, ".")
		var err error
		if len(gv) > 1 {
			var ver int
			ver, err = strconv.Atoi(gv[1])
			w.ispre118 = ver < 18
		}
		if err != nil || len(gv) <= 1 {
			logrus.Info(locale.Loc("guessing_version", nil))
		}

		dim_id := gd.Dimension
		if w.ispre118 {
			logrus.Info(locale.Loc("using_under_118", nil))
			dim_id += 10
		}
		w.Dim = dimension_ids[uint8(dim_id)]
	}

	w.proxy.SendMessage(locale.Loc("use_setname", nil))

	w.ui.Start()
	go func() { // send map item
		select {
		case <-w.ctx.Done():
			return
		default:
			t := time.NewTicker(1 * time.Second)
			for range t.C {
				if w.proxy.Client != nil {
					err := w.proxy.Client.WritePacket(&MAP_ITEM_PACKET)
					if err != nil {
						logrus.Error(err)
						return
					}
				}
			}
		}
	}()

	proxy.AddCommand(utils.IngameCommand{
		Exec: w.setnameCommand,
		Cmd: protocol.Command{
			Name:        "setname",
			Description: locale.Loc("setname_desc", nil),
			Overloads: []protocol.CommandOverload{
				{
					Parameters: []protocol.CommandParameter{
						{
							Name:     "name",
							Type:     protocol.CommandArgTypeString,
							Optional: false,
						},
					},
				},
			},
		},
	})

	proxy.AddCommand(utils.IngameCommand{
		Exec: w.toggleVoid,
		Cmd: protocol.Command{
			Name:        "void",
			Description: locale.Loc("void_desc", nil),
		},
	})
}

func (w *WorldState) ProcessPacketClient(pk packet.Packet) (packet.Packet, bool) {
	forward := true
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
	case *packet.PlayerAuthInput:
		w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
	case *packet.MapInfoRequest:
		if pk.MapID == VIEW_MAP_ID {
			w.ui.SchedRedraw()
			forward = false
		}
	case *packet.ItemStackRequest:
		var requests []protocol.ItemStackRequest
		for _, isr := range pk.Requests {
			for _, sra := range isr.Actions {
				if sra, ok := sra.(*protocol.TakeStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DropStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.PlaceInContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.TakeOutContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == MAP_ITEM_PACKET.Content[0].StackNetworkID {
						continue
					}
				}
			}
			requests = append(requests, isr)
		}
		pk.Requests = requests
	case *packet.MobEquipment:
		if pk.NewItem.Stack.NBTData["map_uuid"] == int64(VIEW_MAP_ID) {
			forward = false
		}
	case *packet.Animate:
		w.ProcessAnimate(pk)
	}
	return pk, forward
}

// stackToItem converts a network ItemStack representation back to an item.Stack.
func stackToItem(it protocol.ItemStack) item.Stack {
	t, ok := world.ItemByRuntimeID(it.NetworkID, int16(it.MetadataValue))
	if !ok {
		t = block.Air{}
	}
	if it.BlockRuntimeID > 0 {
		// It shouldn't matter if it (for whatever reason) wasn't able to get the block runtime ID,
		// since on the next line, we assert that the block is an item. If it didn't succeed, it'll
		// return air anyway.
		b, _ := world.BlockByRuntimeID(uint32(it.BlockRuntimeID))
		if t, ok = b.(world.Item); !ok {
			t = block.Air{}
		}
	}
	//noinspection SpellCheckingInspection
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return nbtconv.ReadItem(it.NBTData, &s)
}

func (w *WorldState) ProcessPacketServer(pk packet.Packet) (packet.Packet, bool) {
	switch pk := pk.(type) {
	case *packet.ChangeDimension:
		w.ProcessChangeDimension(pk)
	case *packet.LevelChunk:
		w.ProcessLevelChunk(pk)

		w.proxy.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{"Count": len(w.chunks), "Name": w.WorldName}, len(w.chunks)))
	case *packet.SubChunk:
		w.ProcessSubChunk(pk)
	case *packet.ContainerOpen:
		if w.experimentInventory {
			// add to open containers
			existing, ok := w.openItemContainers[pk.WindowID]
			if !ok {
				existing = &itemContainer{}
			}
			w.openItemContainers[pk.WindowID] = &itemContainer{
				OpenPacket: pk,
				Content:    existing.Content,
			}
		}
	case *packet.InventoryContent:
		if w.experimentInventory {
			// save content
			existing, ok := w.openItemContainers[byte(pk.WindowID)]
			if !ok {
				if pk.WindowID == 0x0 { // inventory
					w.openItemContainers[byte(pk.WindowID)] = &itemContainer{
						Content: pk,
					}
				}
				break
			}
			existing.Content = pk
		}
	case *packet.ContainerClose:
		if w.experimentInventory {
			switch pk.WindowID {
			case protocol.WindowIDArmour: // todo handle
			case protocol.WindowIDOffHand: // todo handle
			case protocol.WindowIDUI:
			case protocol.WindowIDInventory: // todo handle
			default:
				// find container info
				existing, ok := w.openItemContainers[byte(pk.WindowID)]
				if !ok {
					logrus.Warn(locale.Loc("warn_window_closed_not_open", nil))
					break
				}

				if existing.Content == nil {
					break
				}

				pos := existing.OpenPacket.ContainerPosition
				cp := protocol.SubChunkPos{pos.X() << 4, pos.Z() << 4}

				// create inventory
				inv := inventory.New(len(existing.Content.Content), nil)
				for i, c := range existing.Content.Content {
					item := stackToItem(c.Stack)
					inv.SetItem(i, item)
				}

				// put into subchunk
				nbts := w.blockNBT[cp]
				for i, v := range nbts {
					nbt_pos := protocol.BlockPos{v["x"].(int32), v["y"].(int32), v["z"].(int32)}
					if nbt_pos == pos {
						w.blockNBT[cp][i]["Items"] = nbtconv.InvToNBT(inv)
					}
				}

				w.proxy.SendMessage(locale.Loc("saved_block_inv", nil))

				// remove it again
				delete(w.openItemContainers, byte(pk.WindowID))
			}
		}
	}
	return pk, true
}
