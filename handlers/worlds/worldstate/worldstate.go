package worldstate

import (
	"image"
	"image/draw"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type memoryState struct {
	maps        map[int64]*Map
	chunks      map[world.ChunkPos]*Chunk
	entities    map[entity.RuntimeID]*entity.Entity
	entityLinks map[entity.UniqueID]map[entity.UniqueID]struct{}

	uniqueIDsToRuntimeIDs map[entity.UniqueID]entity.RuntimeID
}

type Chunk struct {
	*chunk.Chunk
	BlockEntities map[cube.Pos]map[string]any
}

func newWorldState() *memoryState {
	return &memoryState{
		maps:        make(map[int64]*Map),
		chunks:      make(map[world.ChunkPos]*Chunk),
		entities:    make(map[entity.RuntimeID]*entity.Entity),
		entityLinks: make(map[entity.UniqueID]map[entity.UniqueID]struct{}),

		uniqueIDsToRuntimeIDs: make(map[entity.UniqueID]entity.RuntimeID),
	}
}

func (w *memoryState) StoreChunk(pos world.ChunkPos, ch *Chunk) {
	w.chunks[pos] = ch
}

func (w *memoryState) StoreMap(m *packet.ClientBoundMapItemData) {
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
	), utils.RGBA2Img(m.Pixels, int(m.Width), int(m.Height)),
		image.Point{},
		draw.Over,
	)
}

func cubePosInChunk(pos cube.Pos) (p world.ChunkPos, sp int16) {
	p[0] = int32(pos.X() >> 4)
	sp = int16(pos.Y() >> 4)
	p[1] = int32(pos.Z() >> 4)
	return
}

func (w *memoryState) StoreEntity(id entity.RuntimeID, es *entity.Entity) {
	w.entities[id] = es
	w.uniqueIDsToRuntimeIDs[es.UniqueID] = es.RuntimeID
}

func (w *memoryState) GetEntity(id entity.RuntimeID) *entity.Entity {
	return w.entities[id]
}

func (w *memoryState) AddEntityLink(el protocol.EntityLink) {
	switch el.Type {
	case protocol.EntityLinkPassenger:
		fallthrough
	case protocol.EntityLinkRider:
		if _, ok := w.entityLinks[el.RiddenEntityUniqueID]; !ok {
			w.entityLinks[el.RiddenEntityUniqueID] = make(map[int64]struct{})
		}
		w.entityLinks[el.RiddenEntityUniqueID][el.RiderEntityUniqueID] = struct{}{}
	case protocol.EntityLinkRemove:
		delete(w.entityLinks[el.RiddenEntityUniqueID], el.RiderEntityUniqueID)
	}
}
