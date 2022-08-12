package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/image/bmp"
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

// draw chunk images to the map image
func (w *WorldState) draw_map() {
	// get the chunk coord bounds
	min := protocol.ChunkPos{}
	max := protocol.ChunkPos{}
	middle := protocol.ChunkPos{}
	for _ch := range w.chunks {
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

	chunks_x := int(max[0] - min[0] + 1)               // how many chunk lengths is x
	chunks_per_line := math.Min(float64(chunks_x), 64) // at max 64 chunks per line
	px_per_chunk := int(128 / chunks_per_line)         // how many pixels does every chunk get

	for i := 0; i < len(w.img.Pix); i++ { // clear canvas
		w.img.Pix[i] = 0
	}

	for _ch := range w.chunks_images {
		px_pos := image.Point{
			X: (int(_ch.X()-middle.X()) * px_per_chunk) + 64,
			Y: (int(_ch.Z()-middle.Z()) * px_per_chunk) + 64,
		}
		if px_pos.In(w.img.Rect) {
			draw.Draw(
				w.img,
				image.Rect(
					px_pos.X,
					px_pos.Y,
					px_pos.X+px_per_chunk,
					px_pos.Y+px_per_chunk,
				),
				w.chunks_images[_ch],
				image.Point{},
				draw.Src,
			)
		}
	}

	{
		buf := bytes.NewBuffer(nil)
		bmp.Encode(buf, w.img)
		os.WriteFile("test.bmp", buf.Bytes(), 0777)
	}
}

var _map_send_lock = sync.Mutex{}

func (w *WorldState) send_map_update(conn *minecraft.Conn) error {
	if !_map_send_lock.TryLock() {
		return nil
	}

	if w.needRedraw {
		w.needRedraw = false
		w.draw_map()
	}

	pixels := make([][]color.RGBA, 128)
	for y := 0; y < 128; y++ {
		pixels[y] = make([]color.RGBA, 128)
		for x := 0; x < 128; x++ {
			pixels[y][x] = w.img.At(x, y).(color.RGBA)
		}
	}

	_map_send_lock.Unlock()
	return conn.WritePacket(&packet.ClientBoundMapItemData{
		MapID:       VIEW_MAP_ID,
		Width:       128,
		Height:      128,
		Pixels:      pixels,
		UpdateFlags: 2,
	})
}
