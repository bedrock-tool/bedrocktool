package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"sync"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
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
var G_state int = state_not_connected
var theme = material.NewTheme(gofont.Collection())

type WorldState struct {
	Chunks       map[protocol.ChunkPos]*packet.LevelChunk
	Entities     map[int64]*packet.AddActor
	BlockUpdates []*packet.UpdateBlock
	_mutex       sync.Mutex
}

var world_state *WorldState = &WorldState{
	Chunks:       make(map[protocol.ChunkPos]*packet.LevelChunk),
	Entities:     make(map[int64]*packet.AddActor),
	BlockUpdates: make([]*packet.UpdateBlock, 0),
	_mutex:       sync.Mutex{},
}

func run_world_downloader() {
	go func() {
		w := app.NewWindow()
		if err := run_gui(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func ProcessChunk(chunk *packet.LevelChunk) {
	world_state._mutex.Lock()
	world_state.Chunks[chunk.Position] = chunk
	world_state._mutex.Unlock()
	G_window.Invalidate()
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

func SetState(state int) {
	G_state = state
	G_window.Invalidate()
}

func handleConn(conn *minecraft.Conn, listener *minecraft.Listener) {
	serverConn, err := minecraft.Dialer{
		TokenSource: G_src,
		ClientData:  conn.ClientData(),
	}.Dial("raknet", "mc.cbps.xyz:19132")
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
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				SetState(state_saving)
				return
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
			case *packet.AddActor:
				ProcessActor(pk)
			case *packet.UpdateBlock:
				ProcessBlockUpdate(pk)
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

func draw_chunks(gtx layout.Context) {
	world_state._mutex.Lock()
	var x_min, x_max, z_min, z_max, chunk_px_size int
	for _, chunk := range world_state.Chunks {
		x_min = int(math.Min(float64(x_min), float64(chunk.Position.X())))
		x_max = int(math.Max(float64(x_max), float64(chunk.Position.X())))
		z_min = int(math.Min(float64(z_min), float64(chunk.Position.Z())))
		z_max = int(math.Max(float64(z_max), float64(chunk.Position.Z())))
	}
	count_x := x_max - x_min + 1
	count_z := z_max - z_min + 1
	count_min := int(math.Min(float64(count_x), float64(count_z)))
	chunk_px_size = int(math.Ceil(math.Sqrt(float64(count_min))))

	for _, chunk := range world_state.Chunks {
		x := int(chunk.Position.X()) - x_min
		z := int(chunk.Position.Z()) - z_min
		draw_rect(gtx, image.Rect(x*chunk_px_size, z*chunk_px_size, (x+1)*chunk_px_size, (z+1)*chunk_px_size), color.NRGBA{0, 0, 0, 255})
	}
	world_state._mutex.Unlock()
}

func draw_working(gtx layout.Context) {
	title := material.H1(theme, fmt.Sprintf("Chunks: %d\nEntities: %d\n", len(world_state.Chunks), len(world_state.Entities)))
	title.Alignment = text.Middle
	title.Color = color.NRGBA{R: 127, G: 0, B: 0, A: 255}
	title.Layout(gtx)

	draw_chunks(gtx)
	//draw_entities(gtx)
}

func draw_saving(gtx layout.Context) {

}

func run_gui(w *app.Window) error {
	G_window = w
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
	}.Listen("raknet", "127.0.0.1:19132")
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
			go handleConn(c.(*minecraft.Conn), listener)
		}
	}()

	for {
		e := <-w.Events()
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
