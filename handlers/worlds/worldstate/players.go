package worldstate

import (
	"fmt"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/utils/resourcepack"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type player struct {
	add                 *packet.AddPlayer
	Position            mgl32.Vec3
	Pitch, Yaw, HeadYaw float32
}

func (w *World) AddPlayer(pk *packet.AddPlayer) {
	w.players[pk.UUID] = &player{
		add:      pk,
		Position: pk.Position,
		Pitch:    pk.Pitch,
		Yaw:      pk.Yaw,
		HeadYaw:  pk.HeadYaw,
	}
}

func (w *World) playersToEntities() (out []resourcepack.EntityPlayer) {
	for _, p := range w.players {
		metadata := protocol.NewEntityMetadata()
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagAlwaysShowName)
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagShowName)
		metadata[protocol.EntityDataKeyName] = p.add.Username
		identifier := fmt.Sprintf("bedrocktool_player:%s", p.add.UUID)
		w.memState.StoreEntity(p.add.EntityRuntimeID, &entity.Entity{
			RuntimeID:  p.add.EntityRuntimeID,
			UniqueID:   int64(p.add.EntityRuntimeID),
			EntityType: identifier,
			Position:   p.Position,
			Pitch:      p.Pitch,
			Yaw:        p.Yaw,
			HeadYaw:    p.HeadYaw,
			Metadata:   metadata,
		})
		out = append(out, resourcepack.EntityPlayer{
			Identifier: identifier,
			UUID:       p.add.UUID,
		})
	}
	return out
}
