package worldstate

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"golang.org/x/exp/maps"
)

type EntityRuntimeID = uint64
type EntityUniqueID = int64

type worldEntities struct {
	entities    map[EntityRuntimeID]*EntityState
	entityLinks map[EntityUniqueID]map[EntityUniqueID]struct{}
	blockNBTs   map[world.ChunkPos]map[cube.Pos]DummyBlock

	uniqueIDsToRuntimeIDs map[EntityUniqueID]EntityRuntimeID
}

func (w *worldEntities) StoreEntity(id EntityRuntimeID, es *EntityState) {
	w.entities[id] = es
}

func (w *worldEntities) GetEntity(id EntityRuntimeID) *EntityState {
	return w.entities[id]
}

func (w *worldEntities) AddEntityLink(el protocol.EntityLink) {
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

func cubePosInChunk(pos cube.Pos) (p world.ChunkPos, sp int16) {
	p[0] = int32(pos.X() >> 4)
	sp = int16(pos.Y() >> 4)
	p[1] = int32(pos.Z() >> 4)
	return
}

func (w *worldEntities) SetBlockNBT(pos cube.Pos, m map[string]any, merge bool) {
	cp, _ := cubePosInChunk(pos)
	chunkNBTs, ok := w.blockNBTs[cp]
	if !ok {
		chunkNBTs = make(map[cube.Pos]DummyBlock)
		w.blockNBTs[cp] = chunkNBTs
	}
	b, ok := chunkNBTs[pos]
	if !ok {
		b = DummyBlock{
			ID:  m["id"].(string),
			NBT: m,
		}
	}

	if merge {
		maps.Copy(b.NBT, m)
	} else {
		b.NBT = m
	}
	chunkNBTs[pos] = b
}
