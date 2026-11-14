package worlds

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// The block from #327 (play.craftersmc.net): its states were registered
// twice and dragonfly panicked with "cannot register the same state twice".
func TestAddCustomBlocksRegistersEachStateOnce(t *testing.T) {
	slab := protocol.BlockEntry{
		Name: "trpixel:end_stone_slab",
		Properties: map[string]any{
			// listed as a property and again through a trait
			"properties": []any{
				map[string]any{"name": "minecraft:vertical_half", "enum": []any{"top", "bottom"}},
			},
			"traits": []any{
				map[string]any{
					"name":           "minecraft:placement_position",
					"enabled_states": map[string]any{"vertical_half": uint8(1)},
				},
			},
		},
	}
	fence := protocol.BlockEntry{
		Name: "test:fence",
		Properties: map[string]any{
			"traits": []any{
				map[string]any{"name": "minecraft:connection", "enabled_states": int32(1)},
			},
		},
	}

	reg := world.NewBlockRegistry()
	// the server sent the slab twice as well
	if err := AddCustomBlocks(reg, []protocol.BlockEntry{slab, slab, fence}); err != nil {
		t.Fatal(err)
	}
	reg.Finalize()

	for _, half := range []string{"top", "bottom"} {
		if _, ok := reg.StateToRuntimeID(slab.Name, map[string]any{"minecraft:vertical_half": half}); !ok {
			t.Errorf("slab state vertical_half=%s missing", half)
		}
	}
	all := map[string]any{
		"minecraft:connection_north": true, "minecraft:connection_south": false,
		"minecraft:connection_west": true, "minecraft:connection_east": false,
	}
	if _, ok := reg.StateToRuntimeID(fence.Name, all); !ok {
		t.Error("fence connection state missing")
	}
}
