package main

import (
	"archive/zip"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"

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
	chunks       map[protocol.ChunkPos]*chunk.Chunk
	entities     map[int64]world.SaveableEntity
	Dim          world.Dimension
	WorldName    string
	ServerName   string
	worldCounter int

	PlayerPos  TPlayerPos
	ClientConn *minecraft.Conn
	ServerConn *minecraft.Conn

	// ui
	ui MapUI
}

func NewWorldState() *WorldState {
	return &WorldState{
		chunks:    make(map[protocol.ChunkPos]*chunk.Chunk),
		Dim:       nil,
		WorldName: "world",
		PlayerPos: TPlayerPos{},
		ui:        NewMapUI(),
	}
}

var setname_command = protocol.Command{
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
}

var black_16x16 = image.NewRGBA(image.Rect(0, 0, 16, 16))

func init() {
	draw.Draw(black_16x16, image.Rect(0, 0, 16, 16), image.Black, image.Point{}, draw.Src)
	register_command(&WorldCMD{})
}

type WorldCMD struct {
	server_address string
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return "download a world from a server" }

func (p *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.server_address, "address", "", "remote server address")
}
func (c *WorldCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *WorldCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
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
	handleConn(ctx, listener, clientConn, serverConn, hostname)

	return 0
}

var dimension_ids = map[int32]world.Dimension{
	0: world.Overworld,
	1: world.Nether,
	2: world.End,
}

func (w *WorldState) ProcessLevelChunk(pk *packet.LevelChunk) {
	ch, err := chunk.NetworkDecode(uint32(pk.HighestSubChunk), pk.RawPayload, int(pk.SubChunkCount), w.Dim.Range())
	if err != nil {
		log.Print(err.Error())
		return
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

		var Offset_table = [][3]int8{
			{0, 0, 0}, {0, 1, 0}, {0, 2, 0}, {0, 3, 0},
			{0, 4, 0}, {0, 5, 0}, {0, 6, 0}, {0, 7, 0},
			{0, 8, 0}, {0, 9, 0}, {0, 10, 0}, {0, 11, 0},
			{0, 12, 0}, {0, 13, 0}, {0, 14, 0}, {0, 15, 0},
			{0, 16, 0}, {0, 17, 0}, {0, 18, 0}, {0, 19, 0},
			{0, 20, 0}, {0, 21, 0}, {0, 22, 0}, {0, 23, 0},
		}

		max := len(Offset_table) - 1
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
		ch := w.chunks[pos]
		if ch == nil {
			fmt.Printf("the server didnt send the chunk before the subchunk!\n")
			continue
		}
		err := ch.ApplySubChunkEntry(uint8(abs_y), &sub)
		if err != nil {
			fmt.Print(err)
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
		send_popup(w.ClientConn, fmt.Sprintf("Zoom: %d", w.ui.zoom))
		w.ui.Send(w)
	}
}

func (w *WorldState) ProcessAddItemActor(pk *packet.AddItemActor) {
	it, ok := world.ItemByRuntimeID(pk.Item.StackNetworkID, int16(pk.Item.Stack.MetadataValue))
	if !ok {
		return
	}
	stack := item.NewStack(it, int(pk.Item.Stack.Count))
	w.entities[pk.EntityUniqueID] = entity.NewItem(stack, mgl64.Vec3{
		float64(pk.Position[0]),
		float64(pk.Position[1]),
		float64(pk.Position[2]),
	})
}

func (w *WorldState) ProcessChangeDimension(pk *packet.ChangeDimension) {
	fmt.Printf("ChangeDimension %d\n", pk.Dimension)
	if len(w.chunks) > 0 {
		w.SaveAndReset()
	} else {
		fmt.Println("Info: Skipping save because the world didnt contain any chunks")
		w.Reset()
	}
	w.Dim = dimension_ids[pk.Dimension]
}

func (w *WorldState) SendMessage(text string) {
	w.ClientConn.WritePacket(&packet.Text{
		TextType: packet.TextTypeSystem,
		Message:  "§8[§bBedrocktool§8]§r " + text,
	})
}

func (w *WorldState) ProcessCommand(pk *packet.CommandRequest) bool {
	cmd := strings.Split(pk.CommandLine, " ")
	if cmd[0] == "/setname" && len(cmd) >= 2 {
		w.WorldName = strings.Join(cmd[1:], " ")
		w.SendMessage(fmt.Sprintf("worldName is now: %s", w.WorldName))
		return true
	}
	return false
}

func (w *WorldState) SetPlayerPos(Position mgl32.Vec3, Pitch, Yaw, HeadYaw float32) {
	w.PlayerPos = TPlayerPos{
		Position: Position,
		Pitch:    Pitch,
		Yaw:      Yaw,
		HeadYaw:  HeadYaw,
	}
}

func (w *WorldState) Reset() {
	w.chunks = make(map[protocol.ChunkPos]*chunk.Chunk)
	w.WorldName = "world"
	w.ui.Reset()
}

// writes the world to a folder, resets all the chunks
func (w *WorldState) SaveAndReset() {
	fmt.Println("Saving world")
	folder := path.Join("worlds", fmt.Sprintf("%s/%s-%d", w.ServerName, w.WorldName, w.worldCounter))
	os.MkdirAll(folder, 0777)
	provider, err := mcdb.New(folder, opt.DefaultCompression)
	if err != nil {
		log.Fatal(err)
	}

	for cp, c := range w.chunks {
		provider.SaveChunk((world.ChunkPos)(cp), c, w.Dim)
	}

	//provider.SaveEntities(w.entities)

	s := provider.Settings()
	s.Spawn = cube.Pos{
		int(w.PlayerPos.Position[0]),
		int(w.PlayerPos.Position[1]),
		int(w.PlayerPos.Position[2]),
	}
	ld := provider.LevelDat()
	for _, gr := range w.ServerConn.GameData().GameRules {
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

	// dont generate
	ld.FlatWorldLayers = `{"biome_id":1,"block_layers":[{"block_data":0,"block_id":0,"count":1},{"block_data":0,"block_id":0,"count":2},{"block_data":0,"block_id":0,"count":1}],"encoding_version":3,"structure_options":null}`
	ld.Generator = 2

	provider.SaveSettings(s)
	provider.Close()
	w.worldCounter += 1

	filename := folder + ".mcworld"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if d.Type().IsRegular() {
			rel := path[len(folder)+1:]
			zwf, _ := zw.Create(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Println(err)
			}
			zwf.Write(data)
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
	zw.Close()
	f.Close()
	fmt.Printf("Saved: %s\n", filename)
	os.RemoveAll(folder)
	w.Reset()
}

func handleConn(ctx context.Context, l *minecraft.Listener, cc, sc *minecraft.Conn, server_name string) {
	w := NewWorldState()
	w.ServerName = server_name
	w.ClientConn = cc
	w.ServerConn = sc

	w.SendMessage("use /setname <worldname>\nto set the world name")

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
			case *packet.ClientCacheStatus:
				pk.Enabled = false
				w.ServerConn.WritePacket(pk)
				skip = true
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
		w.Dim = dimension_ids[gd.Dimension]

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
			case *packet.AddItemActor:
				w.ProcessAddItemActor(pk)
			case *packet.AvailableCommands:
				pk.Commands = append(pk.Commands, setname_command)
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
