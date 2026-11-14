package worldstate

import (
	"context"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func spawnTestWorld(t *testing.T) *World {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w, err := New(ctx, map[int]protocol.DimensionDefinition{}, func(world.ChunkPos, *chunk.Chunk) {})
	if err != nil {
		t.Fatal(err)
	}
	w.SetDimension(world.Overworld)
	br := testRegistry()
	stone, _ := br.StateToRuntimeID("minecraft:stone", nil)
	ch := chunk.New(br, world.Overworld.Range())
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			ch.SetBlock(x, 20, z, 0, stone)
		}
	}
	if err := w.StoreChunk(world.ChunkPos{2, 3}, &Chunk{Chunk: ch, BlockEntities: map[cube.Pos]map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	return w
}

// The first place the player stands in a saved chunk is the spawn.
func TestSpawnFirstStandingPlace(t *testing.T) {
	w := spawnTestWorld(t)
	w.StartPos = mgl32.Vec3{0, 4001.62, 0}
	w.NotePlayerPosition(mgl32.Vec3{0, 30, 0})        // no chunk there
	w.NotePlayerPosition(mgl32.Vec3{40, 4001.62, 50}) // far above the world
	w.NotePlayerPosition(mgl32.Vec3{40, 22.62, 50})
	w.NotePlayerPosition(mgl32.Vec3{41, 22.62, 51})
	if got := w.Spawn(); got != (mgl32.Vec3{40, 22.62, 50}) {
		t.Fatalf("spawn %v", got)
	}
}

// A world the player never stood in starts on the highest block of the
// saved chunk nearest to where the player was.
func TestSpawnFallback(t *testing.T) {
	w := spawnTestWorld(t)
	w.StartPos = mgl32.Vec3{0, 4001.62, 0}
	got := w.Spawn()
	want := mgl32.Vec3{2*16 + 8 + 0.5, 21 + playerEyeHeight, 3*16 + 8 + 0.5}
	if got.Sub(want).Len() > 0.01 {
		t.Fatalf("spawn %v, want %v", got, want)
	}
}
