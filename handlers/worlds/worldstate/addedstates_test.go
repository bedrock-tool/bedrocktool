package worldstate

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func TestAddedStatesRegistry(t *testing.T) {
	reg := world.NewBlockRegistry()
	for _, lit := range []bool{false, true} {
		reg.RegisterBlockState(world.BlockState{Name: "test:lamp", Properties: map[string]any{"test:lit": lit}})
	}
	reg.Finalize()
	lit, ok := reg.StateToRuntimeID("test:lamp", map[string]any{"test:lit": true})
	if !ok {
		t.Fatal("test:lamp not registered")
	}

	added := map[string]map[string]any{"test:lamp": {"minecraft:cardinal_direction": "north"}}
	saved := WithAddedStates(reg, added)

	name, props, ok := saved.RuntimeIDToState(lit)
	want := map[string]any{"test:lit": true, "minecraft:cardinal_direction": "north"}
	if !ok || name != "test:lamp" || !reflect.DeepEqual(props, want) {
		t.Errorf("saved as %s %v, want test:lamp %v", name, props, want)
	}
	// The server's registry is left as it was.
	if _, props, _ := reg.RuntimeIDToState(lit); len(props) != 1 {
		t.Errorf("server registry changed: %v", props)
	}

	// A saved block reads back as the server's block, whichever way it faces.
	for _, dir := range []string{"north", "east"} {
		rid, ok := saved.StateToRuntimeID("test:lamp", map[string]any{"test:lit": true, "minecraft:cardinal_direction": dir})
		if !ok || rid != lit {
			t.Errorf("%s: read back as %d %v, want %d", dir, rid, ok, lit)
		}
	}

	// Vanilla blocks are untouched.
	stone, _ := saved.StateToRuntimeID("minecraft:stone", map[string]any{})
	if _, props, _ := saved.RuntimeIDToState(stone); len(props) != 0 {
		t.Errorf("stone saved with %v", props)
	}

	if WithAddedStates(reg, nil) != reg {
		t.Error("registry wrapped with nothing to add")
	}
}

// A chunk decoded with the wrapped registry is written to disk with the
// added states: chunk.Encode uses the chunk's own registry, not the
// provider's.
func TestAddedStatesSavedInChunks(t *testing.T) {
	reg := world.NewBlockRegistry()
	reg.RegisterBlockState(world.BlockState{Name: "test:statue", Properties: map[string]any{}})
	reg.RegisterBlockState(world.BlockState{Name: "test:plain", Properties: map[string]any{}})
	reg.Finalize()
	statue, _ := reg.StateToRuntimeID("test:statue", map[string]any{})
	plain, _ := reg.StateToRuntimeID("test:plain", map[string]any{})

	saved := WithAddedStates(reg, map[string]map[string]any{"test:statue": {"minecraft:cardinal_direction": "north"}})
	c := chunk.New(saved, cube.Range{-64, 319})
	c.SetBlock(0, 0, 0, 0, statue)
	c.SetBlock(1, 0, 0, 0, plain)

	var palette []byte
	for _, sub := range chunk.Encode(c, chunk.DiskEncoding).SubChunks {
		palette = append(palette, sub...)
	}
	stateAt := func(name string) []byte {
		i := bytes.Index(palette, []byte(name))
		if i < 0 {
			t.Fatalf("%s not in the saved palette", name)
		}
		end := i + 120
		if end > len(palette) {
			end = len(palette)
		}
		if n := bytes.Index(palette[i+len(name):end], []byte("name")); n >= 0 {
			end = i + len(name) + n
		}
		return palette[i:end]
	}
	if !bytes.Contains(stateAt("test:statue"), []byte("minecraft:cardinal_direction")) {
		t.Error("test:statue saved without minecraft:cardinal_direction")
	}
	if bytes.Contains(stateAt("test:plain"), []byte("minecraft:cardinal_direction")) {
		t.Error("test:plain saved with minecraft:cardinal_direction")
	}
}
