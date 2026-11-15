package worldstate

import (
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
)

func TestKeepRemovedSince(t *testing.T) {
	const renderDistance = 48
	now := time.Now()
	w := &World{memState: newWorldState()}
	// Removed long before leaving, close to the player: it died.
	died := &entity.Entity{DeletedDistance: 5, DeletedAt: now.Add(-10 * time.Second)}
	// Removed as the server cleared the world we're leaving.
	cleared := &entity.Entity{DeletedDistance: 7, DeletedAt: now.Add(-200 * time.Millisecond)}
	// Never removed.
	alive := &entity.Entity{DeletedDistance: -1}
	w.memState.entities[1] = died
	w.memState.entities[2] = cleared
	w.memState.entities[3] = alive

	if !entityCull(cleared, renderDistance, nil) {
		t.Fatal("test setup: the cleared entity should look dead before leaving")
	}
	w.KeepRemovedSince(now.Add(-time.Second))

	if entityCull(cleared, renderDistance, nil) {
		t.Error("entity removed while leaving the world is culled")
	}
	if !entityCull(died, renderDistance, nil) {
		t.Error("entity that died earlier is kept")
	}
	if entityCull(alive, renderDistance, nil) {
		t.Error("entity never removed is culled")
	}
}
