package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	state_not_connected = iota
	state_working
	state_saving
	state_done
)

var G_window *app.Window
var G_state int = state_working
var theme = material.NewTheme(gofont.Collection())
var finish_button widget.Clickable

// for player drawing
var chunk_px_size int = 0
var block_coord_top_left protocol.ChunkPos

// the state used for drawing and saving
type WorldState struct {
	Chunks       map[protocol.ChunkPos]*packet.LevelChunk
	SubChunks    map[protocol.ChunkPos]*packet.SubChunk
	Entities     map[int64]*packet.AddActor
	BlockUpdates []*packet.UpdateBlock
	PlayerPos    packet.MovePlayer
	_mutex       sync.Mutex
}

var world_state *WorldState = &WorldState{
	Chunks:       make(map[protocol.ChunkPos]*packet.LevelChunk),
	SubChunks:    make(map[protocol.ChunkPos]*packet.SubChunk),
	Entities:     make(map[int64]*packet.AddActor),
	BlockUpdates: make([]*packet.UpdateBlock, 0),
	PlayerPos:    packet.MovePlayer{},
	_mutex:       sync.Mutex{},
}

func init() {
	register_command("world", "Launch world downloading proxy", world_main)
}

func world_main(args []string) error {
	var target string
	var help bool
	flag.StringVar(&target, "target", "", "target server")
	flag.BoolVar(&help, "help", false, "show help")
	fmt.Printf("%v\n", args)
	flag.CommandLine.Parse(args)
	if help {
		flag.Usage()
		return nil
	}

	if target == "" {
		target = input_server()
	}
	if len(strings.Split(target, ":")) == 1 {
		target += ":19132"
	}

	go func() {
		G_window = app.NewWindow()
		if err := run_gui(target); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}

func ProcessSubChunk(sub_chunk *packet.SubChunk) {

}

func ProcessChunk(chunk *packet.LevelChunk) {
	world_state._mutex.Lock()
	world_state.Chunks[chunk.Position] = chunk
	world_state._mutex.Unlock()
	G_window.Invalidate()
	//os.WriteFile("chunk.chunk", chunk.RawPayload, 0644)
}

func ProcessActor(actor *packet.AddActor) {
	world_state._mutex.Lock()
	world_state.Entities[actor.EntityUniqueID] = actor
	world_state._mutex.Unlock()
	G_window.Invalidate()
}

func ProcessBlockUpdate(update *packet.UpdateBlock) {
	world_state._mutex.Lock()
	world_state.BlockUpdates = append(world_state.BlockUpdates, update)
	world_state._mutex.Unlock()
	G_window.Invalidate()
}

func ProcessMove(player *packet.MovePlayer) {
	world_state.PlayerPos = *player
	G_window.Invalidate()
}

func SetState(state int) {
	G_state = state
	G_window.Invalidate()
}

func handleConn(conn *minecraft.Conn, listener *minecraft.Listener, target string) {
	serverConn, err := minecraft.Dialer{
		TokenSource: G_src,
		ClientData:  conn.ClientData(),
	}.Dial("raknet", target)
	if err != nil {
		panic(err)
	}
	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := conn.StartGame(serverConn.GameData()); err != nil {
			panic(err)
		}
		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()

	SetState(state_working)

	go func() { // client loop
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		defer finish_button.Click()
		for {
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
			}

			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
	}()
	go func() { // server loop
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		defer finish_button.Click()
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}

			switch pk := pk.(type) {
			case *packet.LevelChunk:
				ProcessChunk(pk)
			case *packet.SubChunk:
				ProcessSubChunk(pk)
			case *packet.AddActor:
				ProcessActor(pk)
			case *packet.UpdateBlock:
				ProcessBlockUpdate(pk)
			case *packet.ChunkRadiusUpdated:
				fmt.Printf("ChunkRadiusUpdated: %d\n", pk.ChunkRadius)
			}

			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()
}

func draw_rect(gtx layout.Context, rect image.Rectangle, col color.NRGBA) {
	cl := clip.Rect{Min: rect.Min, Max: rect.Max}.Push(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl.Pop()
}

func layout_chunks(gtx layout.Context) layout.Dimensions {
	world_state._mutex.Lock()
	var x_min, x_max, z_min, z_max int
	for _, chunk := range world_state.Chunks {
		x_min = int(math.Min(float64(x_min), float64(chunk.Position.X())))
		x_max = int(math.Max(float64(x_max), float64(chunk.Position.X())))
		z_min = int(math.Min(float64(z_min), float64(chunk.Position.Z())))
		z_max = int(math.Max(float64(z_max), float64(chunk.Position.Z())))
	}
	x := float64(gtx.Constraints.Max.X - gtx.Constraints.Min.X)
	z := float64(gtx.Constraints.Max.Y - gtx.Constraints.Min.Y)
	count_x := float64(x_max - x_min + 1)
	count_z := float64(z_max - z_min + 1)

	chunk_px_size = int(math.Min(x/count_x, z/count_z))
	block_coord_top_left = protocol.ChunkPos{int32(x_min) * int32(chunk_px_size), int32(z_min) * int32(chunk_px_size)}

	for _, chunk := range world_state.Chunks {
		x := ((int(chunk.Position.X()) - x_min) * chunk_px_size)
		z := ((int(chunk.Position.Z()) - z_min) * chunk_px_size)
		draw_rect(gtx, image.Rect(x, z, x+chunk_px_size, z+chunk_px_size), color.NRGBA{0, 255, 0, 255})
	}

	draw_player_icon(gtx)
	world_state._mutex.Unlock()
	return layout.Dimensions{Size: image.Point{X: chunk_px_size * int(count_x), Y: chunk_px_size * int(count_z)}}
}

func draw_player_icon(gtx layout.Context) {
	player := world_state.PlayerPos

	// calcuate screen position based on chunk position and the chunks screen position
	player_screen := f32.Point{
		X: player.Position.X() - float32(block_coord_top_left.X()),
		Y: player.Position.Z() - float32(block_coord_top_left.Z()),
	}

	op.Affine(f32.Affine2D{}.Rotate(f32.Pt(5, 5), player.HeadYaw*(math.Pi/180)).Offset(player_screen)).Add(gtx.Ops) // rotate and offset relative to first chunk
	draw_rect(gtx, image.Rectangle{image.Point{X: 0, Y: 0}, image.Point{X: 10, Y: 10}}, color.NRGBA{255, 180, 0, 255})
}

func draw_working(gtx layout.Context) {
	layout.Stack{
		Alignment: layout.Center,
	}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				layout.Flexed(0.1, func(gtx layout.Context) layout.Dimensions { // top text
					title := material.H2(theme, fmt.Sprintf("Chunks: %d\n", len(world_state.Chunks)))
					title.Alignment = text.Middle
					title.Color = color.NRGBA{R: 127, G: 0, B: 0, A: 255}
					return title.Layout(gtx)
				}),
				layout.Flexed(0.9, func(gtx layout.Context) layout.Dimensions { // centered chunk view
					return layout.Center.Layout(gtx, layout_chunks)
				}),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			b := material.Button(theme, &finish_button, "Finish")
			b.Color = color.NRGBA{R: 0, G: 127, B: 0, A: 255}
			return b.Layout(gtx)
		}),
	)

	if finish_button.Clicked() {
		SetState(state_saving)
		go begin_save_world(world_state)
	}
}

func begin_save_world(world *WorldState) {

}

func draw_saving(gtx layout.Context) {

}

func run_gui(target string) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}
	defer listener.Close()

	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				log.Fatal(err)
			}
			go handleConn(c.(*minecraft.Conn), listener, target)
		}
	}()

	for {
		e := <-G_window.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)

			switch G_state {
			case state_not_connected:
				title := material.H1(th, fmt.Sprintf("Connect to %s to start", "thelocalserverip"))
				title.Alignment = text.Middle
				title.Color = color.NRGBA{R: 127, G: 0, B: 0, A: 255}
				title.Layout(gtx)
			case state_working:
				draw_working(gtx)
			case state_saving:
				draw_saving(gtx)
			}

			e.Frame(gtx.Ops)
		}
	}
}
