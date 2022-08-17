package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const VIEW_MAP_ID = 0x424242

// packet to tell the client that it has a map with id 0x424242 in the offhand
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

type MapUI struct {
	img           *image.RGBA                       // rendered image
	zoom          int                               // chunks per row
	chunks_images map[protocol.ChunkPos]*image.RGBA // prerendered chunks
	needRedraw    bool                              // when the map has updated this is true
	send_lock     *sync.Mutex
}

func NewMapUI() MapUI {
	return MapUI{
		img:           image.NewRGBA(image.Rect(0, 0, 128, 128)),
		zoom:          64,
		chunks_images: make(map[protocol.ChunkPos]*image.RGBA),
		needRedraw:    true,
		send_lock:     &sync.Mutex{},
	}
}

// Reset resets the map to inital state
func (m *MapUI) Reset() {
	m.chunks_images = make(map[protocol.ChunkPos]*image.RGBA)
	m.needRedraw = true
}

// ChangeZoom adds to the zoom value and goes around to 32 once it hits 128
func (m *MapUI) ChangeZoom() {
	if m.zoom >= 128 {
		m.zoom = 32 // min
	} else {
		m.zoom += 32
	}
	m.SchedRedraw()
}

// SchedRedraw tells the map to redraw the next time its sent
func (m *MapUI) SchedRedraw() {
	m.needRedraw = true
}

// draw chunk images to the map image
func (m *MapUI) Redraw(w *WorldState) {
	// get the chunk coord bounds
	min := protocol.ChunkPos{}
	max := protocol.ChunkPos{}
	middle := protocol.ChunkPos{}
	for _ch := range m.chunks_images {
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
		if _ch.X() == int32(w.PlayerPos.Position.X()/16) && _ch.Z() == int32(w.PlayerPos.Position.Z()/16) {
			middle = _ch
		}
	}

	chunks_x := int(max[0] - min[0] + 1)                            // how many chunk lengths is x coordinate
	chunks_per_line := math.Min(float64(chunks_x), float64(m.zoom)) // either zoom or how many there actually are
	px_per_chunk := int(128 / chunks_per_line)                      // how many pixels does every chunk get

	for i := 0; i < len(m.img.Pix); i++ { // clear canvas
		m.img.Pix[i] = 0
	}

	for _ch := range m.chunks_images {
		px_pos := image.Point{ // bottom left corner of the chunk on the map
			X: (int(_ch.X()-middle.X()) * px_per_chunk) + 64,
			Y: (int(_ch.Z()-middle.Z()) * px_per_chunk) + 64,
		}
		if px_pos.In(m.img.Rect) {
			draw.Draw(
				m.img,
				image.Rect(
					px_pos.X,
					px_pos.Y,
					px_pos.X+px_per_chunk,
					px_pos.Y+px_per_chunk,
				),
				m.chunks_images[_ch],
				image.Point{},
				draw.Src,
			)
		}
	}
	/*
		{
			buf := bytes.NewBuffer(nil)
			bmp.Encode(buf, m.img)
			os.WriteFile("test.bmp", buf.Bytes(), 0777)
		}
	*/
}

// send
func (m *MapUI) Send(w *WorldState) error {
	if !m.send_lock.TryLock() {
		return nil // dont send if send is in progress
	}

	// redraw if needed
	if m.needRedraw {
		m.needRedraw = false
		m.Redraw(w)
	}

	// (ugh)
	pixels := make([][]color.RGBA, 128)
	for y := 0; y < 128; y++ {
		pixels[y] = make([]color.RGBA, 128)
		for x := 0; x < 128; x++ {
			pixels[y][x] = m.img.At(x, y).(color.RGBA)
		}
	}

	m.send_lock.Unlock()
	return w.ClientConn.WritePacket(&packet.ClientBoundMapItemData{
		MapID:       VIEW_MAP_ID,
		Width:       128,
		Height:      128,
		Pixels:      pixels,
		UpdateFlags: 2,
	})
}

func (m *MapUI) SetChunk(pos protocol.ChunkPos, ch *chunk.Chunk) {
	var img *image.RGBA
	if ch != nil {
		img = Chunk2Img(ch)
	} else {
		img = black_16x16
	}
	m.send_lock.Lock() // dont send while adding a chunk
	m.chunks_images[pos] = img
	m.send_lock.Unlock()
	m.SchedRedraw()
}
