package worldstate

import (
	"fmt"
	"maps"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/utils/resourcepack"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PlayerEntityType marks a player in the world's entities until it is saved.
// Players can't be stored as minecraft:player, so on save each kept player
// becomes its own entity type (bedrocktool_player:<uuid>) drawn with its
// skin, and the rest are dropped.
const PlayerEntityType = "minecraft:player"

// PlayerEyeHeight is how far above its feet the server puts a player in
// movement packets (AddPlayer has the feet).
const PlayerEyeHeight = 1.62

type player struct {
	UUID      uuid.UUID
	Username  string
	RuntimeID entity.RuntimeID
}

// AddPlayer stores another player as an entity, so the usual entity packets
// (movement, metadata, removal) keep it up to date.
func (w *World) AddPlayer(pk *packet.AddPlayer) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	w.players[pk.UUID] = &player{
		UUID:      pk.UUID,
		Username:  pk.Username,
		RuntimeID: pk.EntityRuntimeID,
	}

	ent := w.memState.GetEntity(pk.EntityRuntimeID)
	if ent == nil {
		ent = &entity.Entity{
			RuntimeID:            pk.EntityRuntimeID,
			Inventory:            make(map[byte]map[byte]protocol.ItemInstance),
			Metadata:             make(map[uint32]any),
			Properties:           make(map[string]any),
			PropertiesOverridden: make(map[string]any),
		}
	}
	ent.UniqueID = pk.AbilityData.EntityUniqueID
	if ent.UniqueID == 0 {
		ent.UniqueID = int64(pk.EntityRuntimeID)
	}
	ent.EntityType = PlayerEntityType
	ent.Position = pk.Position
	ent.Pitch = pk.Pitch
	ent.Yaw = pk.Yaw
	ent.HeadYaw = pk.HeadYaw
	ent.Velocity = pk.Velocity
	ent.DeletedDistance = -1
	ent.LastTeleport = 0
	maps.Copy(ent.Metadata, pk.EntityMetadata)
	w.memState.StoreEntity(pk.EntityRuntimeID, ent)
}

// SavedPlayer is a player kept by preparePlayers, with what its entity
// definition needs.
type SavedPlayer struct {
	resourcepack.EntityPlayer
	Metadata protocol.EntityMetadata
}

// preparePlayers gives each player keep accepts its own entity type and
// drops the others from the world. Called with stateLock held.
func (w *World) preparePlayers(keep func(uuid.UUID) bool) (out []SavedPlayer) {
	byRuntimeID := make(map[entity.RuntimeID]*player, len(w.players))
	for _, p := range w.players {
		byRuntimeID[p.RuntimeID] = p
	}
	for rid, ent := range w.memState.entities {
		if ent.EntityType != PlayerEntityType {
			continue
		}
		p := byRuntimeID[rid]
		if p == nil || keep == nil || !keep(p.UUID) {
			delete(w.memState.entities, rid)
			continue
		}
		identifier := fmt.Sprintf("bedrocktool_player:%s", p.UUID)
		ent.EntityType = identifier
		if _, ok := ent.Metadata[protocol.EntityDataKeyName]; !ok {
			ent.Metadata[protocol.EntityDataKeyName] = p.Username
			ent.Metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagAlwaysShowName)
			ent.Metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagShowName)
		}
		out = append(out, SavedPlayer{
			EntityPlayer: resourcepack.EntityPlayer{Identifier: identifier, UUID: p.UUID},
			Metadata:     ent.Metadata,
		})
	}
	return out
}
