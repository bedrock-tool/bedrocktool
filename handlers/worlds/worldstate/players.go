package worldstate

import (
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
		metadata := protocol.NewEntityMetadata()
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagAlwaysShowName)
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagShowName)
		metadata[protocol.EntityDataKeyName] = p.add.Username
		w.currState().StoreEntity(p.add.EntityRuntimeID, &entity.Entity{
			RuntimeID:  p.add.EntityRuntimeID,
			UniqueID:   int64(p.add.EntityRuntimeID),
			EntityType: "player:" + p.add.UUID.String(),
			Position:   p.Position,
			Pitch:      p.Pitch,
			Yaw:        p.Yaw,
			HeadYaw:    p.HeadYaw,
			Metadata:   metadata,
		})
	}
}
