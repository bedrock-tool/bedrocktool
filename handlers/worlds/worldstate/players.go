package worldstate

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type player struct {
	add                 *packet.AddPlayer
	Position            mgl32.Vec3
	Pitch, Yaw, HeadYaw float32
}

type worldPlayers struct {
	players map[uuid.UUID]*player
}

func (w *World) AddPlayer(pk *packet.AddPlayer) {
	_, ok := w.players.players[pk.UUID]
	if ok {
		//logrus.Debugf("duplicate player %v", pk)
		return
	}
	w.players.players[pk.UUID] = &player{
		add:      pk,
		Position: pk.Position,
		Pitch:    pk.Pitch,
		Yaw:      pk.Yaw,
		HeadYaw:  pk.HeadYaw,
	}
}

func (w *World) playersToEntities() {
	for _, p := range w.players.players {
		w.worldEntities.StoreEntity(p.add.EntityRuntimeID, &EntityState{
			RuntimeID:  p.add.EntityRuntimeID,
			UniqueID:   int64(p.add.EntityRuntimeID),
			EntityType: "bedrocktool:fake_player",
			Position:   p.Position,
			Pitch:      p.Pitch,
			Yaw:        p.Yaw,
			HeadYaw:    p.HeadYaw,
		})
	}
}
