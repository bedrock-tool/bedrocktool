package worldstate

import (
	"image"
	"image/draw"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/thomaso-mirodin/intmath/i32"
)

type worldStateDefer struct {
	chunks map[world.ChunkPos]*chunk.Chunk
	worldEntities
	maps map[int64]*Map
}

func (w *worldStateDefer) StoreChunk(pos world.ChunkPos, ch *chunk.Chunk, blockNBT map[cube.Pos]DummyBlock) {
	w.chunks[pos] = ch
	w.blockNBTs[pos] = blockNBT
}

func (w *worldStateDefer) StoreMap(m *packet.ClientBoundMapItemData) {
	return // not finished yet
	m1, ok := w.maps[m.MapID]
	if !ok {
		m1 = &Map{
			MapID:     m.MapID,
			Height:    128,
			Width:     128,
			Scale:     1,
			Dimension: 0,
			ZCenter:   m.Origin.Z(),
			XCenter:   m.Origin.X(),
		}
		w.maps[m.MapID] = m1
	}
	draw.Draw(&image.RGBA{
		Pix:    m1.Colors[:],
		Rect:   image.Rect(0, 0, int(m.Width), int(m.Height)),
		Stride: int(m.Width) * 4,
	}, image.Rect(
		int(m.XOffset), int(m.YOffset),
		int(m.Width), int(m.Height),
	), utils.RGBA2Img(
		m.Pixels,
		image.Rect(
			0, 0,
			int(m.Width), int(m.Height),
		),
	), image.Point{}, draw.Over)
}

func (w *worldStateDefer) cullChunks() {
	for key, ch := range w.chunks {
		var empty = true
		for _, sub := range ch.Sub() {
			if !sub.Empty() {
				empty = false
				break
			}
		}
		if empty {
			delete(w.chunks, key)
		}
	}
}

func (w *worldStateDefer) ApplyTo(w2 worldStateInterface, around cube.Pos, radius int32, cf func(world.ChunkPos, *chunk.Chunk)) {
	w.cullChunks()
	for cp, c := range w.chunks {
		dist := i32.Sqrt(i32.Pow(cp.X()-int32(around.X()/16), 2) + i32.Pow(cp.Z()-int32(around.Z()/16), 2))
		blockNBT := w.blockNBTs[cp]
		if dist <= radius || radius < 0 {
			w2.StoreChunk(cp, c, blockNBT)
			cf(cp, c)
		} else {
			cf(cp, nil)
		}
	}

	for k, es := range w.entities {
		x := int(es.Position[0])
		z := int(es.Position[2])
		dist := i32.Sqrt(i32.Pow(int32(x-around.X()), 2) + i32.Pow(int32(z-around.Z()), 2))
		e2 := w2.GetEntity(k)
		if e2 != nil || dist < radius*16 || radius < 0 {
			w2.StoreEntity(k, es)
		}
	}
}
