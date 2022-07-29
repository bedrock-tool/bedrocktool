package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/df-mc/dragonfly/server/world/chunk"
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
