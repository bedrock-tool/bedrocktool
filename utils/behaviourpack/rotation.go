package behaviourpack

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

const cardinalDirection = "minecraft:cardinal_direction"

// cardinalRotations is the Y rotation of each cardinal direction, as in
// https://wiki.bedrock.dev/blocks/custom-heads#applying-block-rotation.
var cardinalRotations = []struct {
	direction string
	y         float32
}{
	{"north", 0},
	{"west", 90},
	{"south", 180},
	{"east", -90},
}

// cardinalDefault is the value a saved block takes for a cardinal_direction
// state the server never sent: the trait's default.
const cardinalDefault = "north"

// cardinalYOffset turns the placement direction around, so a placed block's
// front faces the player while north (what saved blocks get) stays
// unrotated, as the server showed them.
const cardinalYOffset = 180

// addCardinalRotation lets a block be placed facing north, east, south or
// west: it enables minecraft:cardinal_direction with the placement_direction
// trait and adds a permutation rotating the block for each direction. It
// leaves alone a block without minecraft:geometry (a cube textured by
// blocks.json, which a transformation doesn't turn), one that already
// rotates (a placement trait, or permutations on cardinal_direction or
// facing_direction), one whose permutations set their own transformation,
// and one whose format is too old for traits. It says whether it added the
// rotation.
func addCardinalRotation(b *MinecraftBlock, version string) bool {
	if version < "1.20.20" || !hasGeometry(b) {
		return false
	}
	for name := range b.Description.Traits {
		if name == "minecraft:placement_direction" || name == "minecraft:placement_position" {
			return false
		}
	}
	if _, ok := b.Description.States[cardinalDirection]; ok {
		return false
	}
	for _, p := range b.Permutations {
		if strings.Contains(p.Condition, "cardinal_direction") || strings.Contains(p.Condition, "facing_direction") {
			return false
		}
		// Later permutations replace a component, so ours would undo the
		// block's own transformation.
		if _, ok := p.Components["minecraft:transformation"]; ok {
			return false
		}
	}

	if b.Description.Traits == nil {
		b.Description.Traits = make(map[string]Trait)
	}
	b.Description.Traits["minecraft:placement_direction"] = Trait{
		"enabled_states":    []string{cardinalDirection},
		"y_rotation_offset": cardinalYOffset,
	}

	base, _ := b.Components["minecraft:transformation"].(map[string][]float32)
	for _, r := range cardinalRotations {
		transform := map[string][]float32{"rotation": {0, r.y, 0}}
		if base != nil {
			// Keep the block's own translation, scale and rotation.
			transform = maps.Clone(base)
			rotation := []float32{0, 0, 0}
			copy(rotation, base["rotation"])
			rotation[1] += r.y
			transform["rotation"] = rotation
		}
		b.Permutations = append(b.Permutations, permutation{
			Condition:  fmt.Sprintf("q.block_state('%s') == '%s'", cardinalDirection, r.direction),
			Components: map[string]any{"minecraft:transformation": transform},
		})
	}
	return true
}

// hasGeometry says whether the block, or any of its permutations, is drawn
// with a model of its own.
func hasGeometry(b *MinecraftBlock) bool {
	if _, ok := b.Components["minecraft:geometry"]; ok {
		return true
	}
	for _, p := range b.Permutations {
		if _, ok := p.Components["minecraft:geometry"]; ok {
			return true
		}
	}
	return false
}

// stateCondition matches a permutation condition on one integer state, in
// either query form: q.block_state('ns:s') == 2, query.block_property(...).
var stateCondition = regexp.MustCompile(`^\s*(?:q|query)\.block_(?:state|property)\(\s*'([^']+)'\s*\)\s*==\s*(\d+)\s*$`)

// placeRotation is how a block that keeps its direction in a state of its
// own (Hive's custom:facing_direction) turns: the state, and the value for
// each Y rotation (Yaw) or for each axis a pillar lies along (Axis).
type placeRotation struct {
	State string         `json:"state"`
	Yaw   map[string]int `json:"yaw,omitempty"`
	Axis  map[string]int `json:"axis,omitempty"`
}

// findPlaceRotation finds a state the block's permutations rotate it by,
// value by value. The server sets such a state and the game never does, so
// a block placed in the saved world would never turn: the pack's script
// sets it on placement instead (see placementScript).
func findPlaceRotation(b *MinecraftBlock) *placeRotation {
	rotations := map[int][3]float32{}
	state := ""
	for _, p := range b.Permutations {
		m := stateCondition.FindStringSubmatch(p.Condition)
		if m == nil {
			continue
		}
		if state != "" && m[1] != state {
			return nil
		}
		state = m[1]
		value, _ := strconv.Atoi(m[2])
		var rot [3]float32
		if t, ok := p.Components["minecraft:transformation"].(map[string][]float32); ok {
			copy(rot[:], t["rotation"])
		}
		rotations[value] = rot
	}
	if state == "" || len(rotations) < 2 {
		return nil
	}
	norm := func(d float32) int { return ((int(d) % 360) + 360) % 360 }

	yaw := map[string]int{}
	for v, r := range rotations {
		if norm(r[0]) == 0 && norm(r[2]) == 0 {
			key := strconv.Itoa(norm(r[1]))
			if _, dup := yaw[key]; dup {
				continue
			}
			yaw[key] = v
		}
	}
	if len(yaw) >= 2 {
		return &placeRotation{State: state, Yaw: yaw}
	}

	axis := map[string]int{}
	for v, r := range rotations {
		x, y, z := norm(r[0]), norm(r[1]), norm(r[2])
		switch {
		case y != 0:
			return nil
		case x == 0 && z == 0:
			axis["y"] = v
		case (x == 90 || x == 270) && z == 0:
			axis["z"] = v // turned about X: lies along Z
		case x == 0 && (z == 90 || z == 270):
			axis["x"] = v // turned about Z: lies along X
		}
	}
	if len(axis) >= 2 {
		return &placeRotation{State: state, Axis: axis}
	}
	return nil
}

// placementScriptFile is where the pack's placement script goes; main.js
// imports it.
const placementScriptFile = "rotate_on_place.js"

// placementScript sets each listed block's direction state when a player
// places it: facing the player for blocks turning about Y (as blocks with
// cardinal rotation do), or along the axis the player looks for pillars.
func placementScript(blocks map[string]*placeRotation) string {
	table, _ := json.Marshal(blocks)
	return `import { world } from "@minecraft/server";

// Blocks keeping their direction in a state of their own, set by the server
// they were saved from: block id -> {state, yaw: {degrees: value}, axis: {x|y|z: value}}.
const rotations = ` + string(table) + `;

// main.js imports this first: nothing here may stop the rest from loading.
try {
world.afterEvents.playerPlaceBlock.subscribe(({ block, player }) => {
  const r = rotations[block.typeId];
  if (!r) return;
  let value;
  if (r.yaw) {
    // Face the player: looking south (yaw 0) leaves the block unturned.
    const deg = (((-Math.round(player.getRotation().y / 90) * 90) % 360) + 360) % 360;
    value = r.yaw[String(deg)];
  } else if (r.axis) {
    const v = player.getViewDirection();
    const axis = Math.abs(v.y) > 0.7 ? "y" : Math.abs(v.x) > Math.abs(v.z) ? "x" : "z";
    value = r.axis[axis];
  }
  if (value === undefined) return;
  try {
    block.setPermutation(block.permutation.withState(r.state, value));
  } catch (e) {
    console.warn("rotate " + block.typeId + ": " + e);
  }
});
} catch (e) {
  console.warn("rotate on place: " + e);
}
`
}
