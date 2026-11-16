package behaviourpack

import (
	"encoding/json"
	"regexp"
	"strconv"
)

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

	// A block turning about Y can also lean the same way for every value,
	// as Hive's wall attachments do (X 90 or 270): only Y tells them apart.
	tiltX, tiltZ := -1, -1
	for _, r := range rotations {
		x, z := norm(r[0]), norm(r[2])
		if tiltX == -1 {
			tiltX, tiltZ = x, z
		} else if x != tiltX || z != tiltZ {
			tiltX, tiltZ = 0, 0
			break
		}
	}
	yaw := map[string]int{}
	for v, r := range rotations {
		if norm(r[0]) == tiltX && norm(r[2]) == tiltZ {
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
