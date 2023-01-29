package world

import (
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type entityState struct {
	RuntimeId  uint64
	UniqueId   int64
	EntityType string

	Position         mgl32.Vec3
	Pitch, Yaw       float32
	HeadYaw, BodyYaw float32
	Velocity         mgl32.Vec3

	Metadata map[uint32]any
}

type serverEntity struct {
	world.Entity
	EntityType serverEntityType
}

type serverEntityType struct {
	world.SaveableEntityType
	Encoded string
	NBT     map[string]any
}

func (t serverEntityType) EncodeEntity() string {
	return t.Encoded
}

func (t serverEntityType) DecodeNBT(m map[string]any) world.Entity {
	return nil // not implemented, and never should
}

func (t serverEntityType) EncodeNBT(e *serverEntity) map[string]any {
	return t.NBT
}

func (t serverEntityType) BBox(e world.Entity) cube.BBox {
	return cube.Box(0, 0, 0, 1, 1, 1)
}

func (e serverEntity) Type() world.EntityType {
	return e.EntityType
}

func (w *WorldState) processAddActor(pk *packet.AddActor) {
	e, ok := w.entities[pk.EntityRuntimeID]
	if !ok {
		e = &entityState{
			RuntimeId:  pk.EntityRuntimeID,
			UniqueId:   pk.EntityUniqueID,
			EntityType: pk.EntityType,
			Metadata:   make(map[uint32]any),
		}
		w.entities[pk.EntityRuntimeID] = e

		w.bp.AddEntity(behaviourpack.EntityIn{
			Identifier: pk.EntityType,
			Attr:       pk.Attributes,
		})
	}

	e.Position = pk.Position
	e.Pitch = pk.Pitch
	e.Yaw = pk.Yaw
	e.BodyYaw = pk.BodyYaw
	e.HeadYaw = pk.HeadYaw
	e.Velocity = pk.Velocity

	for k, v := range pk.EntityMetadata {
		e.Metadata[k] = v
	}
}

func (w *WorldState) ProcessEntityPackets(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.AddActor:
		w.processAddActor(pk)
	case *packet.RemoveActor:
		delete(w.entities, uint64(pk.EntityUniqueID))
	case *packet.SetActorData:
		e, ok := w.entities[pk.EntityRuntimeID]
		if ok {
			for k, v := range pk.EntityMetadata {
				e.Metadata[k] = v
			}
		}
	case *packet.SetActorMotion:
		e, ok := w.entities[pk.EntityRuntimeID]
		if ok {
			e.Velocity = pk.Velocity
		}
	case *packet.MoveActorDelta:
		e, ok := w.entities[pk.EntityRuntimeID]
		if ok {
			if pk.Flags&packet.MoveActorDeltaFlagHasX != 0 {
				e.Position[0] = pk.Position[0]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasY != 0 {
				e.Position[1] = pk.Position[1]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasZ != 0 {
				e.Position[2] = pk.Position[2]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasRotX != 0 {
				e.Pitch = pk.Rotation.X()
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasRotY != 0 {
				e.Yaw = pk.Rotation.Y()
			}
			//if pk.Flags&packet.MoveActorDeltaFlagHasRotZ != 0 {
			// no roll
			//}
		}
	case *packet.MoveActorAbsolute:
		e, ok := w.entities[pk.EntityRuntimeID]
		if ok {
			e.Position = pk.Position
			e.Pitch = pk.Rotation.X()
			e.Yaw = pk.Rotation.Y()
		}
	}
	return pk
}
