package behaviourpack

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// blockJSON adds the block to a pack and returns its generated JSON, decoded.
func blockJSON(t *testing.T, bp *Pack, entry protocol.BlockEntry) map[string]any {
	t.Helper()
	bp.AddBlock(entry)
	data, err := json.Marshal(bp.blocks[entry.Name])
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out["minecraft:block"].(map[string]any)
}

// TestBlockKeptAsSent checks the pack adds no rotation of its own: a block
// the server sent without one stays without one, and one with Mojang's
// placement trait keeps it as sent.
func TestBlockKeptAsSent(t *testing.T) {
	for _, c := range []struct {
		name  string
		props map[string]any
	}{
		{"no rotation", map[string]any{
			"components": map[string]any{
				"minecraft:geometry": map[string]any{"identifier": "geometry.plain"},
			},
		}},
		{"placement_direction", map[string]any{
			"traits": []any{map[string]any{
				"name":           "placement_direction",
				"enabled_states": map[string]any{"facing_direction": uint8(0), "cardinal_direction": uint8(1)},
			}},
			"components": map[string]any{
				"minecraft:geometry": map[string]any{"identifier": "geometry.own"},
			},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			bp := New("test")
			block := blockJSON(t, bp, protocol.BlockEntry{Name: "test:own", Properties: c.props})
			if perms, ok := block["permutations"]; ok {
				t.Errorf("permutations added: %v", perms)
			}
			traits, _ := block["description"].(map[string]any)["traits"].(map[string]any)
			_, sent := c.props["traits"]
			if len(traits) > 0 != sent {
				t.Errorf("traits = %v", traits)
			}
			if states, _ := block["description"].(map[string]any)["states"].(map[string]any); len(states) > 0 {
				t.Errorf("states added: %v", states)
			}
			if len(bp.placeRotations) != 0 || bp.HasScript() {
				t.Errorf("placement script for a block without its own rotation state: %+v", bp.placeRotations)
			}
		})
	}
}

// facingPerms is Hive's custom:facing_direction permutations, a rotation
// for each value.
func facingPerms(rotations map[int][3]int32) []any {
	var perms []any
	for v, r := range rotations {
		perms = append(perms, map[string]any{
			"condition": "query.block_property('custom:facing_direction') == " + strconv.Itoa(v),
			"components": map[string]any{
				"minecraft:geometry":       map[string]any{"identifier": "geometry.thing"},
				"minecraft:transformation": map[string]any{"RX": r[0], "RY": r[1], "RZ": r[2]},
			},
		})
	}
	return perms
}

func TestPlaceRotation(t *testing.T) {
	bp := New("test")
	// As Hive's founder_ad:elderspore_mature: 0, 180, 90, 270 degrees about Y.
	bp.AddBlock(protocol.BlockEntry{Name: "test:spore", Properties: map[string]any{
		"properties":   []any{map[string]any{"name": "custom:facing_direction", "enum": []int32{0, 1, 2, 3}}},
		"permutations": facingPerms(map[int][3]int32{0: {0, 0, 0}, 1: {0, 2, 0}, 2: {0, 1, 0}, 3: {0, 3, 0}}),
	}})
	// A pillar: Y, turned about X (along Z), turned about Z (along X).
	bp.AddBlock(protocol.BlockEntry{Name: "test:pillar", Properties: map[string]any{
		"properties":   []any{map[string]any{"name": "custom:facing_direction", "enum": []int32{0, 1, 2}}},
		"permutations": facingPerms(map[int][3]int32{0: {0, 0, 0}, 1: {1, 0, 0}, 2: {0, 0, 1}}),
	}})

	// As Hive's founder_ad:eldermoss_vine_attachment_top: leaning 90 degrees
	// about X for every value, turned about Y.
	bp.AddBlock(protocol.BlockEntry{Name: "test:vine", Properties: map[string]any{
		"properties":   []any{map[string]any{"name": "custom:facing_direction", "enum": []int32{0, 1, 2, 3}}},
		"permutations": facingPerms(map[int][3]int32{0: {1, 0, 0}, 1: {1, 2, 0}, 2: {1, 1, 0}, 3: {1, 3, 0}}),
	}})

	want := map[string]*placeRotation{
		"test:spore":  {State: "custom:facing_direction", Yaw: map[string]int{"0": 0, "180": 1, "90": 2, "270": 3}},
		"test:pillar": {State: "custom:facing_direction", Axis: map[string]int{"y": 0, "z": 1, "x": 2}},
		"test:vine":   {State: "custom:facing_direction", Yaw: map[string]int{"0": 0, "180": 1, "90": 2, "270": 3}},
	}
	if !reflect.DeepEqual(bp.placeRotations, want) {
		t.Errorf("placeRotations = %+v, want %+v", bp.placeRotations, want)
	}
	if !bp.HasScript() {
		t.Fatal("no script for blocks rotated on placement")
	}
	if main := bp.mainScript(); !strings.HasPrefix(main, `import "./rotate_on_place.js";`) {
		t.Errorf("main.js doesn't import the placement script: %q", main)
	}
	script := placementScript(bp.placeRotations)
	for _, s := range []string{"playerPlaceBlock", `"test:spore"`, `"custom:facing_direction"`, "withState"} {
		if !strings.Contains(script, s) {
			t.Errorf("placement script lacks %s", s)
		}
	}

	// A state that isn't a rotation is left alone.
	other := New("test")
	other.AddBlock(protocol.BlockEntry{Name: "test:lamp", Properties: map[string]any{
		"permutations": []any{
			map[string]any{"condition": "q.block_state('test:lit') == 0", "components": map[string]any{}},
			map[string]any{"condition": "q.block_state('test:lit') == 1", "components": map[string]any{}},
		},
	}})
	if len(other.placeRotations) != 0 || other.HasScript() {
		t.Errorf("test:lamp taken for a rotating block: %+v", other.placeRotations)
	}
}

func TestManifest(t *testing.T) {
	bp := New("test")
	if bp.Manifest.Header.MinimumGameVersion != MinEngineVersion {
		t.Errorf("min_engine_version %v, want %v for block traits", bp.Manifest.Header.MinimumGameVersion, MinEngineVersion)
	}
	bp.AddDependency("a", [3]int{1, 0, 0})
	bp.AddDependency("a", [3]int{1, 0, 0})
	if len(bp.Manifest.Dependencies) != 1 {
		t.Errorf("dependency added twice: %v", bp.Manifest.Dependencies)
	}
}

// Custom entities the server sent are saved invulnerable.
func TestEntityInvulnerable(t *testing.T) {
	bp := New("test")
	bp.AddEntity("hivehub:npc_questmaster", nil, nil, nil)
	bp.AddEntity("minecraft:zombie", nil, nil, nil)
	ds, ok := bp.entities["hivehub:npc_questmaster"].MinecraftEntity.Components["minecraft:damage_sensor"].(map[string]any)
	if !ok {
		t.Fatal("custom entity has no damage_sensor")
	}
	triggers := ds["triggers"].([]any)
	if trig := triggers[0].(map[string]any); trig["cause"] != "all" || trig["deals_damage"] != "no" {
		t.Errorf("damage_sensor = %v", ds)
	}
	if _, ok := bp.entities["minecraft:zombie"]; ok {
		t.Error("vanilla entity defined")
	}
}
