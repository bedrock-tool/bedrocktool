package worldstate

import (
	"path"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type EntityRuntimeID = uint64
type EntityUniqueID = int64

type worldEntities struct {
	entities    map[EntityRuntimeID]*EntityState
	entityLinks map[EntityUniqueID]map[EntityUniqueID]struct{}
	blockNBTs   map[world.ChunkPos]map[cube.Pos]DummyBlock
}

func (w *worldEntities) StoreEntity(id EntityRuntimeID, es *EntityState) {
	w.entities[id] = es
}

func (w *worldEntities) GetEntity(id EntityRuntimeID) (*EntityState, bool) {
	e, ok := w.entities[id]
	return e, ok
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

func (w *World) saveEntities(exclude []string) error {
	w.l.Lock()
	defer w.l.Unlock()

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.worldEntities.entities {
		var ignore bool
		for _, ex := range exclude {
			if ok, err := path.Match(ex, es.EntityType); ok {
				logrus.Debugf("Excluding: %s %v", es.EntityType, es.Position)
				ignore = true
				break
			} else if err != nil {
				logrus.Warn(err)
			}
		}
		if !ignore {
			cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
			links := maps.Keys(w.worldEntities.entityLinks[es.UniqueID])
			chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity(links))
		}
	}

	for cp, v := range chunkEntities {
		err := w.provider.StoreEntities(cp, w.dimension, v)
		if err != nil {
			logrus.Error(err)
		}
	}

	return nil
}

func (w *World) saveBlockNBTs(dim world.Dimension) error {
	for cp, v := range w.worldEntities.blockNBTs {
		vv := make(map[cube.Pos]world.Block, len(v))
		for p, db := range v {
			vv[p] = &db
		}
		err := w.provider.StoreBlockNBTs(cp, dim, vv)
		if err != nil {
			return err
		}
	}
	return nil
}
