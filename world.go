package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"

	_ "github.com/df-mc/dragonfly/server/block" // to load blocks
)

type TPlayerPos struct {
	Position mgl32.Vec3
	Pitch    float32
	Yaw      float32
	HeadYaw  float32
}

// the state used for drawing and saving

type WorldState struct {
	ispre118     bool
	voidgen      bool
	chunks       map[protocol.ChunkPos]*chunk.Chunk
	entities     map[int64]world.SaveableEntity
	blockNBT     map[protocol.SubChunkPos][]map[string]any
	Dim          world.Dimension
	WorldName    string
	ServerName   string
	worldCounter int
	packs        map[string]*resource.Pack

	PlayerPos  TPlayerPos
	ClientConn *minecraft.Conn
	ServerConn *minecraft.Conn

	log *logrus.Logger

	// ui
	ui MapUI
}

func NewWorldState() *WorldState {
	return &WorldState{
		chunks:    make(map[protocol.ChunkPos]*chunk.Chunk),
		blockNBT:  make(map[protocol.SubChunkPos][]map[string]any),
		entities:  make(map[int64]world.SaveableEntity),
		Dim:       nil,
		WorldName: "world",
		PlayerPos: TPlayerPos{},
		ui:        NewMapUI(),
	}
}

type IngameCommand struct {
	exec func(w *WorldState, cmdline []string) bool
	cmd  protocol.Command
}

var IngameCommands = map[string]IngameCommand{
	"setname": {
		exec: setnameCommand,
		cmd: protocol.Command{
			Name:        "setname",
			Description: "set user defined name for this world",
			Overloads: []protocol.CommandOverload{
				{
					Parameters: []protocol.CommandParameter{
						{
							Name:     "name",
							Type:     protocol.CommandArgTypeFilepath,
							Optional: false,
						},
					},
				},
			},
		},
	},
	"void": {
		exec: toggleVoid,
		cmd: protocol.Command{
			Name:        "void",
			Description: "toggle if void generator should be used",
		},
	},
}

func setnameCommand(w *WorldState, cmdline []string) bool {
	w.WorldName = strings.Join(cmdline, " ")
	send_message(w.ClientConn, fmt.Sprintf("worldName is now: %s", w.WorldName))
	return true
}

func toggleVoid(w *WorldState, cmdline []string) bool {
	w.voidgen = !w.voidgen
	send_message(w.ClientConn, fmt.Sprintf("using void generator: %t", w.voidgen))
	return true
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
	cs := crc32.ChecksumIEEE([]byte(a))
	if cs != 0x9747c04f {
		a += "T" + "A" + "M" + "P" + "E" + "R" + "E" + "D"
	}
	register_command(&WorldCMD{})
}

type WorldCMD struct {
	server_address string
	packs          bool
	enableVoid     bool
	log            *logrus.Logger
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return "download a world from a server" }

func (p *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.server_address, "address", "", "remote server address")
	f.BoolVar(&p.packs, "packs", false, "save resourcepacks to the worlds")
	f.BoolVar(&p.enableVoid, "void", true, "if false, saves with default flat generator")
}

func (c *WorldCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + SERVER_ADDRESS_HELP
}

func (c *WorldCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	c.log = logrus.New()

	server_address, hostname, err := server_input(c.server_address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	listener, clientConn, serverConn, err := create_proxy(ctx, server_address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	c.handleConn(ctx, listener, clientConn, serverConn, hostname)

	return 0
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

func (w *WorldState) ProcessLevelChunk(pk *packet.LevelChunk) {
	ch, blockNBTs, err := chunk.NetworkDecode(uint32(pk.HighestSubChunk), pk.RawPayload, int(pk.SubChunkCount), w.Dim.Range(), w.ispre118)
	if err != nil {
		log.Print(err.Error())
		return
	}
	if blockNBTs != nil {
		w.blockNBT[protocol.SubChunkPos{
			pk.Position.X(), 0, pk.Position.Z(),
		}] = blockNBTs
	}

	existing := w.chunks[pk.Position]
	if existing == nil {
		w.chunks[pk.Position] = ch
		w.ui.SetChunk(pk.Position, nil)
	}

	if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLegacy {
		w.ui.SetChunk(pk.Position, ch)
	} else {
		// request all the subchunks

		max := w.Dim.Range().Height() / 16
		if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLimited {
			max = int(pk.HighestSubChunk)
		}

		w.ServerConn.WritePacket(&packet.SubChunkRequest{
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
		abs_x := pk.Position[0] + int32(sub.Offset[0])
		abs_y := pk.Position[1] + int32(sub.Offset[1])
		abs_z := pk.Position[2] + int32(sub.Offset[2])
		pos := protocol.ChunkPos{abs_x, abs_z}
		pos3 := protocol.SubChunkPos{abs_x, abs_y, abs_z}
		ch := w.chunks[pos]
		if ch == nil {
			fmt.Printf("the server didnt send the chunk before the subchunk!\n")
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(abs_y), &sub)
		if err != nil {
			fmt.Print(err)
		}
		if blockNBT != nil {
			w.blockNBT[pos3] = blockNBT
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
		w.ui.Send(w)
	}
}

func (w *WorldState) ProcessChangeDimension(pk *packet.ChangeDimension) {
	fmt.Printf("ChangeDimension %d\n", pk.Dimension)
	if len(w.chunks) > 0 {
		w.SaveAndReset()
	} else {
		fmt.Println("Info: Skipping save because the world didnt contain any chunks")
		w.Reset()
	}
	dim_id := pk.Dimension
	if w.ispre118 {
		dim_id += 10
	}
	w.Dim = dimension_ids[uint8(dim_id)]
}

func (w *WorldState) ProcessCommand(pk *packet.CommandRequest) bool {
	cmd := strings.Split(pk.CommandLine, " ")
	name := cmd[0][1:]
	if h, ok := IngameCommands[name]; ok {
		return h.exec(w, cmd[1:])
	}
	return false
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
		w.ui.Send(w)
	}
}

func (w *WorldState) Reset() {
	w.chunks = make(map[protocol.ChunkPos]*chunk.Chunk)
	w.WorldName = fmt.Sprintf("world-%d", w.worldCounter)
	w.ui.Reset()
}

// writes the world to a folder, resets all the chunks
func (w *WorldState) SaveAndReset() {
	fmt.Printf("Saving world %s\n", w.WorldName)

	// open world
	folder := path.Join("worlds", fmt.Sprintf("%s/%s", w.ServerName, w.WorldName))
	os.RemoveAll(folder)
	os.MkdirAll(folder, 0o777)

	provider, err := mcdb.New(w.log, folder, opt.DefaultCompression)
	if err != nil {
		log.Fatal(err)
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
			fmt.Fprintln(os.Stderr, err)
		}
	}

	// write metadata
	s := provider.Settings()
	s.Spawn = cube.Pos{
		int(w.PlayerPos.Position[0]),
		int(w.PlayerPos.Position[1]),
		int(w.PlayerPos.Position[2]),
	}
	s.Name = w.WorldName

	// set gamerules
	ld := provider.LevelDat()
	gd := w.ServerConn.GameData()
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
			fmt.Printf("unknown gamerule: %s\n", gr.Name)
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

	for k, p := range w.packs {
		fmt.Printf("Adding resource pack: %s\n", k)
		pack_folder := path.Join(folder, "resource_packs", k)
		os.MkdirAll(pack_folder, 0o755)
		data := make([]byte, p.Len())
		p.ReadAt(data, 0)
		unpack_zip(bytes.NewReader(data), int64(len(data)), pack_folder)
	}

	// zip it
	filename := folder + ".mcworld"

	if err := zip_folder(filename, folder); err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Saved: %s\n", filename)
	os.RemoveAll(folder)
	w.Reset()
}

func (c *WorldCMD) handleConn(ctx context.Context, l *minecraft.Listener, cc, sc *minecraft.Conn, server_name string) {
	var err error
	w := NewWorldState()
	w.ServerName = server_name
	w.ClientConn = cc
	w.ServerConn = sc
	w.voidgen = c.enableVoid
	w.log = c.log

	if c.packs {
		fmt.Println("reformatting packs")
		go func() {
			w.packs, err = w.getPacks()
		}()
	}

	{ // check game version
		gd := w.ServerConn.GameData()
		gv := strings.Split(gd.BaseGameVersion, ".")
		if len(gv) > 1 {
			var ver int
			ver, err = strconv.Atoi(gv[1])
			w.ispre118 = ver < 18
		}
		if err != nil || len(gv) <= 1 {
			fmt.Println("couldnt determine game version, assuming > 1.18")
		}
		if w.ispre118 {
			fmt.Println("using legacy (< 1.18)")
		}
	}

	send_message(w.ClientConn, "use /setname <worldname>\nto set the world name")

	G_exit = append(G_exit, func() {
		w.SaveAndReset()
	})

	done := make(chan struct{})

	go func() { // client loop
		defer func() { done <- struct{}{} }()
		for {
			skip := false
			pk, err := w.ClientConn.ReadPacket()
			if err != nil {
				return
			}

			switch pk := pk.(type) {
			case *packet.MovePlayer:
				w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
			case *packet.PlayerAuthInput:
				w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
			case *packet.MapInfoRequest:
				if pk.MapID == VIEW_MAP_ID {
					w.ui.Send(w)
					skip = true
				}
			case *packet.MobEquipment:
				if pk.NewItem.Stack.NBTData["map_uuid"] == int64(VIEW_MAP_ID) {
					skip = true
				}
			case *packet.Animate:
				w.ProcessAnimate(pk)
			case *packet.CommandRequest:
				skip = w.ProcessCommand(pk)
			}

			if !skip {
				if err := w.ServerConn.WritePacket(pk); err != nil {
					if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
						_ = l.Disconnect(w.ClientConn, disconnect.Error())
					}
					return
				}
			}
		}
	}()

	go func() { // send map item
		select {
		case <-ctx.Done():
			return
		default:
			for {
				time.Sleep(1 * time.Second)
				err := w.ClientConn.WritePacket(&MAP_ITEM_PACKET)
				if err != nil {
					return
				}
			}
		}
	}()

	go func() { // server loop
		defer w.ServerConn.Close()
		defer l.Disconnect(w.ClientConn, "connection lost")
		defer func() { done <- struct{}{} }()

		gd := w.ServerConn.GameData()
		dim_id := gd.Dimension
		if w.ispre118 {
			dim_id += 10
		}
		w.Dim = dimension_ids[uint8(dim_id)]

		for {
			pk, err := w.ServerConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = l.Disconnect(w.ClientConn, disconnect.Error())
				}
				return
			}

			switch pk := pk.(type) {
			case *packet.ChangeDimension:
				w.ProcessChangeDimension(pk)
			case *packet.LevelChunk:
				w.ProcessLevelChunk(pk)
				w.ui.Send(w)
				send_popup(w.ClientConn, fmt.Sprintf("%d chunks loaded\nname: %s", len(w.chunks), w.WorldName))
			case *packet.SubChunk:
				w.ProcessSubChunk(pk)
			case *packet.AvailableCommands:
				for _, ic := range IngameCommands {
					pk.Commands = append(pk.Commands, ic.cmd)
				}
			}

			if err := w.ClientConn.WritePacket(pk); err != nil {
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return
	case <-done:
		return
	}
}
