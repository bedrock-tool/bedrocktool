package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
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
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// the state used for drawing and saving
type WorldState struct {
	Provider  *mcdb.Provider
	Dim       world.Dimension
	WorldName string
	PlayerPos packet.MovePlayer
	img       *image.RGBA
	chunks    map[protocol.ChunkPos]interface{}
	_mutex    sync.Mutex
}

var world_state *WorldState = &WorldState{
	Provider:  nil,
	Dim:       nil,
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

func ProcessChunk(pk *packet.LevelChunk) {
	ch, err := chunk.NetworkDecode(uint32(pk.HighestSubChunk), pk.RawPayload, int(pk.SubChunkCount), cube.Range{-64, 320})
	if err != nil {
		log.Printf(err.Error())
		return
	}

	// save the current chunk
	world_state.Provider.SaveChunk((world.ChunkPos)(pk.Position), ch, world_state.Dim)

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
	folder := path.Join("worlds", fmt.Sprintf("%s-dim-%d", world_state.WorldName, pk.Dimension))
	provider, err := mcdb.New(folder, opt.DefaultCompression)
	if err != nil {
		log.Fatal(err)
	}
	if world_state.Provider != nil {
		world_state.Provider.Close()
	}
	world_state.Provider = provider
	world_state.Dim = dimension_ids[pk.Dimension]
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
		world_state.Provider.Close()
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

		gd := serverConn.GameData()
		ProcessChangeDimension(&packet.ChangeDimension{
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
