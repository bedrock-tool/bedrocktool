package worldstate

import (
	"testing"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestMountSeats(t *testing.T) {
	w := newTestWorld(t)
	chair := protocol.NewEntityMetadata()
	chair[protocol.EntityDataKeyScale] = float32(2)
	w.StoreEntity(1, &entity.Entity{RuntimeID: 1, UniqueID: 100, EntityType: "hivemounts:recliner", Metadata: chair})

	// a player sitting on it, the link sent with AddPlayer
	meta := protocol.NewEntityMetadata()
	meta[protocol.EntityDataKeySeatOffset] = mgl32.Vec3{0, 1.32 + 0.4, 0}
	id := uuid.New()
	w.AddPlayer(&packet.AddPlayer{
		UUID: id, EntityRuntimeID: 2, AbilityData: protocol.AbilityData{EntityUniqueID: 200}, EntityMetadata: meta,
		EntityLinks: []protocol.EntityLink{{RiddenEntityUniqueID: 100, RiderEntityUniqueID: 200, Type: protocol.EntityLinkRider}},
	})

	w.stateLock.Lock()
	w.preparePlayers(func(uuid.UUID) bool { return true })
	seats := w.mountSeatsLocked()
	w.stateLock.Unlock()

	s := seats["hivemounts:recliner"]
	if len(s) != 1 {
		t.Fatalf("seats %v", seats)
	}
	// (1.72 - 1.22) / 2
	if y := s[0].Y(); y < 0.249 || y > 0.251 {
		t.Errorf("seat y %v, want 0.25", y)
	}
	r := rideable(s)
	if r["seat_count"] != 1 {
		t.Errorf("rideable %v", r)
	}

	// the rider left out: no seats
	w2 := newTestWorld(t)
	w2.StoreEntity(1, &entity.Entity{RuntimeID: 1, UniqueID: 100, EntityType: "hivemounts:recliner", Metadata: protocol.NewEntityMetadata()})
	w2.AddPlayer(&packet.AddPlayer{UUID: id, EntityRuntimeID: 2, AbilityData: protocol.AbilityData{EntityUniqueID: 200}, EntityMetadata: protocol.NewEntityMetadata(),
		EntityLinks: []protocol.EntityLink{{RiddenEntityUniqueID: 100, RiderEntityUniqueID: 200, Type: protocol.EntityLinkRider}}})
	w2.stateLock.Lock()
	w2.preparePlayers(nil)
	if s := w2.mountSeatsLocked(); len(s) != 0 {
		t.Errorf("seats for a rider not saved: %v", s)
	}
	w2.stateLock.Unlock()
}
