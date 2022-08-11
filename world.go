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
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// the state used for drawing and saving
type WorldState struct {
	Provider  *mcdb.Provider // provider for the current world
	chunks    map[protocol.ChunkPos]*chunk.Chunk
	Dim       world.Dimension
	WorldName string
	PlayerPos packet.MovePlayer

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
	PlayerPos:     packet.MovePlayer{},
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
	_, server = server_input(ctx, server)

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
	// perhaps just update the current chunk instead of overwrite
	w.chunks[pk.Position] = ch

	if pk.SubChunkCount == 0 { // no sub chunks = no blocks known
		w.chunks_images[pk.Position] = black_16x16
	} else {
		w.chunks_images[pk.Position] = Chunk2Img(ch)
	}
	w.needRedraw = true
}

func (w *WorldState) ProcessSubChunk(pk *packet.SubChunk) {
	pos := protocol.ChunkPos{pk.Position.X(), pk.Position.Z()}
	// get or create chunk ptr
	ch := w.chunks[pos]
	if ch == nil { // create an empty chunk if for some reason the server didnt send the chunk before the subchunk
		fmt.Printf("the server didnt send the chunk before the subchunk!\n")
		ch = chunk.New(0, w.Dim.Range())
		w.chunks[pos] = ch
	}
	// add the new subs
	//err := ch.ApplySubChunkEntries(int16(pk.Position.Y()), pk.SubChunkEntries)
	//if err != nil {
	//	fmt.Print(err)
	//}
	// redraw the chunk
	w.chunks_images[pos] = Chunk2Img(ch)
	w.needRedraw = true
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

func (w *WorldState) ProcessMove(player *packet.MovePlayer) {
	w.PlayerPos = *player
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
				__world_state.ProcessMove(pk)
			case *packet.MapInfoRequest:
				if pk.MapID == VIEW_MAP_ID {
					__world_state.send_map_update(conn)
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
				send_popup(conn, fmt.Sprintf("%d chunks loaded\n", len(__world_state.chunks)))
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
