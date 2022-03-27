package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const VIEW_MAP_ID = 0x424242

var MAP_ITEM_PACKET packet.InventoryContent = packet.InventoryContent{
	WindowID: 119,
	Content: []protocol.ItemInstance{
		{
			StackNetworkID: 1,
			Stack: protocol.ItemStack{
				ItemType: protocol.ItemType{
					NetworkID:     420,
					MetadataValue: 0,
				},
				BlockRuntimeID: 0,
				Count:          1,
				NBTData: map[string]interface{}{
					"map_uuid": int64(VIEW_MAP_ID),
				},
			},
		},
	},
}

// the state used for drawing and saving
type WorldState struct {
	Dimension *mcdb.Provider
	WorldName string
	PlayerPos packet.MovePlayer
	img       *image.RGBA
	chunks    map[protocol.ChunkPos]interface{}
	_mutex    sync.Mutex
}

var world_state *WorldState = &WorldState{
	Dimension: nil,
	WorldName: "world",
	PlayerPos: packet.MovePlayer{},
	img:       image.NewRGBA(image.Rect(0, 0, 128, 128)),
	chunks:    make(map[protocol.ChunkPos]interface{}),
	_mutex:    sync.Mutex{},
}

var tmp_chunk_cache map[protocol.ChunkPos]*chunk.Chunk = make(map[protocol.ChunkPos]*chunk.Chunk)

func init() {
	register_command("world", "Launch world downloading proxy", world_main)
}

func world_main(ctx context.Context, args []string) error {
	var server string
	flag.StringVar(&server, "server", "", "target server")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	_, server = server_input(ctx, server)

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("Listening on %s\n", listener.Addr())

	c, err := listener.Accept()
	if err != nil {
		log.Fatal(err)
	}

	// not a goroutine, only 1 client at a time
	handleConn(ctx, c.(*minecraft.Conn), listener, server)

	return nil
}

func draw_chunk(pos protocol.ChunkPos, ch *chunk.Chunk) {
	if world_state.chunks[pos] != nil {
		return
	}

	world_state.chunks[pos] = true
	min := protocol.ChunkPos{}
	max := protocol.ChunkPos{}
	for _ch := range world_state.chunks {
		if _ch.X() < min.X() {
			min[0] = _ch.X()
		}
		if _ch.Z() < min.Z() {
			min[1] = _ch.Z()
		}
		if _ch.X() > max.X() {
			max[0] = _ch.X()
		}
		if _ch.Z() > max.Z() {
			max[1] = _ch.Z()
		}
	}

	px_per_chunk := 128 / int(max[0]-min[0]+1)

	world_state.img.Pix = make([]uint8, world_state.img.Rect.Dx()*world_state.img.Rect.Dy()*4)

	for _ch := range world_state.chunks {
		px_pos := image.Point{X: int(_ch.X() - min.X()), Y: int(_ch.Z() - min.Z())}
		draw.Draw(
			world_state.img,
			image.Rect(
				px_pos.X*px_per_chunk,
				px_pos.Y*px_per_chunk,
				(px_pos.X+1)*px_per_chunk,
				(px_pos.Y+1)*px_per_chunk,
			),
			image.White,
			image.Point{},
			draw.Src,
		)
	}

	{
		f, _ := os.Create("test.png")
		png.Encode(f, world_state.img)
		f.Close()
	}
}

var _map_send_lock = false

func get_map_update() *packet.ClientBoundMapItemData {
	if _map_send_lock {
		return nil
	}
	_map_send_lock = true

	pixels := make([][]color.RGBA, 128)
	for y := 0; y < 128; y++ {
		pixels[y] = make([]color.RGBA, 128)
		for x := 0; x < 128; x++ {
			pixels[y][x] = world_state.img.At(x, y).(color.RGBA)
		}
	}

	_map_send_lock = false
	return &packet.ClientBoundMapItemData{
		MapID:       VIEW_MAP_ID,
		Width:       128,
		Height:      128,
		Pixels:      pixels,
		UpdateFlags: 2,
	}
}

func ProcessChunk(pk *packet.LevelChunk) {
	ch, err := chunk.NetworkDecode(uint32(pk.HighestSubChunk), pk.RawPayload, int(pk.SubChunkCount), cube.Range{-64, 320})
	if err != nil {
		log.Fatal(err)
	}

	if world_state.Dimension == nil {
		tmp_chunk_cache[pk.Position] = ch
	} else {
		if len(tmp_chunk_cache) > 0 { // write the cached to the world and empty it
			for pos, ch := range tmp_chunk_cache {
				world_state.Dimension.SaveChunk((world.ChunkPos)(pos), ch)
			}
			tmp_chunk_cache = make(map[protocol.ChunkPos]*chunk.Chunk)
			fmt.Printf("dumped cache %d\n", len(tmp_chunk_cache))
		}
		// save the current chunk
		world_state.Dimension.SaveChunk((world.ChunkPos)(pk.Position), ch)
	}
	draw_chunk(pk.Position, ch)
}

var gamemode_ids = map[int32]world.GameMode{
	0: world.GameModeSurvival,
	1: world.GameModeCreative,
	2: world.GameModeAdventure,
	3: world.GameModeSpectator,
	4: world.GameModeCreative,
}

var dimension_ids = map[int32]world.Dimension{
	0: world.Overworld,
	1: world.Nether,
	2: world.End,
}

var difficulty_ids = map[int32]world.Difficulty{
	0: world.DifficultyPeaceful,
	1: world.DifficultyEasy,
	2: world.DifficultyNormal,
	3: world.DifficultyHard,
}

func ProcessChangeDimension(pk *packet.ChangeDimension) {
	fmt.Printf("ChangeDimension %d\n", pk.Dimension)
	dimension, err := mcdb.New(path.Join("worlds", fmt.Sprintf("%s-dim-%d", world_state.WorldName, pk.Dimension)), dimension_ids[pk.Dimension])
	if err != nil {
		log.Fatal(err)
	}
	if world_state.Dimension != nil {
		world_state.Dimension.Close()
	}
	world_state.Dimension = dimension
	world_state.chunks = make(map[protocol.ChunkPos]interface{})
}

func ProcessMove(player *packet.MovePlayer) {
	world_state.PlayerPos = *player
}

func spawn_conn(ctx context.Context, conn *minecraft.Conn, serverConn *minecraft.Conn) error {
	errs := make(chan error, 2)
	go func() {
		errs <- conn.StartGame(serverConn.GameData())
	}()
	go func() {
		errs <- serverConn.DoSpawn()
	}()

	// wait for both to finish
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				return fmt.Errorf("failed to start game: %s", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("connection cancelled")
		}
	}
	return nil
}

func handleConn(ctx context.Context, conn *minecraft.Conn, listener *minecraft.Listener, target string) {
	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	fmt.Printf("Connecting to %s\n", target)
	serverConn, err := minecraft.Dialer{
		TokenSource: G_src,
		ClientData:  conn.ClientData(),
		PacketFunc:  packet_func,
	}.DialContext(ctx, "raknet", target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to %s: %s\n", target, err)
		return
	}

	if err := spawn_conn(ctx, conn, serverConn); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to spawn: %s\n", err)
		return
	}

	G_exit = func() {
		serverConn.Close()
		conn.Close()
		listener.Close()
		world_state.Dimension.Close()
	}

	done := make(chan struct{})

	go func() { // client loop
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		defer func() { done <- struct{}{} }()
		for {
			skip := false
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}

			switch _pk := pk.(type) {
			case *packet.RequestChunkRadius:
				pk = &packet.RequestChunkRadius{ // rewrite packet to send a bigger radius
					ChunkRadius: 32,
				}
			case *packet.MovePlayer:
				ProcessMove(_pk)
			case *packet.MapInfoRequest:
				fmt.Printf("MapInfoRequest: %d\n", _pk.MapID)
				if _pk.MapID == VIEW_MAP_ID {
					if update_pk := get_map_update(); update_pk != nil {
						conn.WritePacket(update_pk)
					}
					skip = true
				}
			}

			if !skip {
				if err := serverConn.WritePacket(pk); err != nil {
					if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
						_ = listener.Disconnect(conn, disconnect.Error())
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
				err := conn.WritePacket(&MAP_ITEM_PACKET)
				if err != nil {
					return
				}
			}
		}
	}()

	go func() { // server loop
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		defer func() { done <- struct{}{} }()
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}

			switch pk := pk.(type) {
			case *packet.ChangeDimension:
				ProcessChangeDimension(pk)
			case *packet.LevelChunk:
				ProcessChunk(pk)
				if _pk := get_map_update(); _pk != nil {
					if err := conn.WritePacket(_pk); err != nil {
						panic(err)
					}
				}
				send_popup(conn, fmt.Sprintf("%d chunks loaded", len(world_state.chunks)))
			}

			if err := conn.WritePacket(pk); err != nil {
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
