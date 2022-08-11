package main

import (
	"image"
	"image/color"
	"image/draw"

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

// draw chunk images to the map image
func (w *WorldState) draw_map() {
	// get the chunk coord bounds
	min := protocol.ChunkPos{}
	max := protocol.ChunkPos{}
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
	}

	px_per_chunk := 128 / int(max[0]-min[0]+1)

	for i := 0; i < len(w.img.Pix); i++ { // clear canvas
		w.img.Pix[i] = 0
	}

	for _ch := range w.chunks {
		px_pos := image.Point{X: int(_ch.X() - min.X()), Y: int(_ch.Z() - min.Z())}
		draw.Draw(
			w.img,
			image.Rect(
				px_pos.X*px_per_chunk,
				px_pos.Y*px_per_chunk,
				(px_pos.X+1)*px_per_chunk,
				(px_pos.Y+1)*px_per_chunk,
			),
			w.chunks_images[_ch],
			image.Point{},
			draw.Src,
		)
	}
}

var _map_send_lock = false

func (w *WorldState) send_map_update(conn *minecraft.Conn) error {
	if _map_send_lock {
		return nil
	}
	_map_send_lock = true

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

	_map_send_lock = false
	return conn.WritePacket(&packet.ClientBoundMapItemData{
		MapID:       VIEW_MAP_ID,
		Width:       128,
		Height:      128,
		Pixels:      pixels,
		UpdateFlags: 2,
	})
}
