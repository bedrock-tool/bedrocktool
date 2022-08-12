package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"log"
	"net"
	"os"
	"path"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type TPlayerPos struct {
	Position mgl32.Vec3
	Pitch    float32
	Yaw      float32
	HeadYaw  float32
}

// the state used for drawing and saving
type WorldState struct {
	Provider  *mcdb.Provider // provider for the current world
	chunks    map[protocol.ChunkPos]*chunk.Chunk
	Dim       world.Dimension
	WorldName string
	PlayerPos TPlayerPos

	// ui
	img           *image.RGBA
	chunks_images map[protocol.ChunkPos]*image.RGBA // rendered those chunks
	needRedraw    bool
}

var __world_state *WorldState = &WorldState{
	Provider:      nil,
	chunks:        make(map[protocol.ChunkPos]*chunk.Chunk),
	Dim:           nil,
	WorldName:     "world",
	PlayerPos:     TPlayerPos{},
	img:           image.NewRGBA(image.Rect(0, 0, 128, 128)),
	chunks_images: make(map[protocol.ChunkPos]*image.RGBA),
	needRedraw:    true,
}

var black_16x16 = image.NewRGBA(image.Rect(0, 0, 16, 16))

func init() {
	draw.Draw(black_16x16, image.Rect(0, 0, 16, 16), image.Black, image.Point{}, draw.Src)
	register_command("world", "Launch world downloading proxy", world_main)
}

func world_main(ctx context.Context, args []string) error {
	var server string

	if len(args) >= 1 {
		server = args[0]
		args = args[1:]
	}
	_, server = server_input(server)

	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

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

var dimension_ids = map[int32]world.Dimension{
	0: world.Overworld,
	1: world.Nether,
	2: world.End,
}

func (w *WorldState) ProcessLevelChunk(pk *packet.LevelChunk, serverConn *minecraft.Conn) {
	ch, err := chunk.NetworkDecode(uint32(pk.HighestSubChunk), pk.RawPayload, int(pk.SubChunkCount), w.Dim.Range())
	if err != nil {
		log.Print(err.Error())
		return
	}
	existing := w.chunks[pk.Position]
	if existing == nil {
		w.chunks[pk.Position] = ch
		w.chunks_images[pk.Position] = black_16x16
	}

	if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLegacy {
		w.chunks_images[pk.Position] = Chunk2Img(ch)
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

		serverConn.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(w.Dim.EncodeDimension()),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: Offset_table[:max],
		})
	}
	w.needRedraw = true
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
		err := ch.ApplySubChunkEntry(int(abs_y), &sub)
		if err != nil {
			fmt.Print(err)
		}
		pos_to_redraw[pos] = true
	}

	// redraw the chunks
	for pos := range pos_to_redraw {
		w.chunks_images[pos] = Chunk2Img(w.chunks[pos])
		w.needRedraw = true
	}
}

func (w *WorldState) ProcessChangeDimension(pk *packet.ChangeDimension) {
	fmt.Printf("ChangeDimension %d\n", pk.Dimension)
	folder := path.Join("worlds", fmt.Sprintf("%s-dim-%d", w.WorldName, pk.Dimension))
	provider, err := mcdb.New(folder, opt.DefaultCompression)
	if err != nil {
		log.Fatal(err)
	}
	if w.Provider != nil {
		w.Provider.Close()
	}
	w.Provider = provider
	w.Dim = dimension_ids[pk.Dimension]
	w.chunks = make(map[protocol.ChunkPos]*chunk.Chunk)
	w.chunks_images = make(map[protocol.ChunkPos]*image.RGBA)
	w.needRedraw = true
}

func (w *WorldState) SetPlayerPos(Position mgl32.Vec3, Pitch, Yaw, HeadYaw float32) {
	w.PlayerPos = TPlayerPos{
		Position: Position,
		Pitch:    Pitch,
		Yaw:      Yaw,
		HeadYaw:  HeadYaw,
	}
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

	G_exit = append(G_exit, func() {
		serverConn.Close()
		conn.Close()
		listener.Close()
		println("Closing Provider")
		__world_state.Provider.Close()
	})

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

			switch pk := pk.(type) {
			case *packet.MovePlayer:
				__world_state.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
			case *packet.PlayerAuthInput:
				__world_state.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
			case *packet.MapInfoRequest:
				if pk.MapID == VIEW_MAP_ID {
					__world_state.send_map_update(conn)
					skip = true
				}
			case *packet.ClientCacheStatus:
				pk.Enabled = false
				serverConn.WritePacket(pk)
				skip = true
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

		gd := serverConn.GameData()
		__world_state.ProcessChangeDimension(&packet.ChangeDimension{
			Dimension: gd.Dimension,
			Position:  gd.PlayerPosition,
			Respawn:   false,
		})

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
				__world_state.ProcessChangeDimension(pk)
			case *packet.LevelChunk:
				__world_state.ProcessLevelChunk(pk, serverConn)
				__world_state.send_map_update(conn)
				p := __world_state.PlayerPos.Position
				send_popup(conn, fmt.Sprintf("%d chunks loaded\npos: %.02f %.02f %.02f\n", len(__world_state.chunks), p.X(), p.Y(), p.Z()))
			case *packet.SubChunk:
				__world_state.ProcessSubChunk(pk)
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
