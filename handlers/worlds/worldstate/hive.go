package worldstate

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// HiveTextEntity is Hive's floating name tag. Hive spawns sheep only as
// hitboxes and floating name tags, and its own pack turns each sheep into
// hive:text, so a saved Hive world stores them as hive:text, which Hive's
// resource pack draws as just the name.
const HiveTextEntity = "hive:text"

// HiveTextComponents is hive:text's behaviour, as in Hive's pack: no size,
// can't be hurt or pushed, name always shown. Gravity off, so tags floating
// in the air stay there.
var HiveTextComponents = map[string]any{
	"minecraft:collision_box": map[string]any{"width": 0.0, "height": 0.0},
	"minecraft:damage_sensor": map[string]any{"triggers": map[string]any{"deals_damage": false}},
	"minecraft:health":        map[string]any{"value": 1, "max": 1, "min": 0},
	"minecraft:nameable":      map[string]any{"always_show": true},
	"minecraft:pushable":      map[string]any{"is_pushable": false, "is_pushable_by_piston": false},
	"minecraft:physics":       map[string]any{"has_gravity": false, "has_collision": false},
	"minecraft:push_through":  1,
	"minecraft:is_stackable":  map[string]any{},
}

// convertHiveSheep turns every sheep into hive:text, dropping what only a
// sheep has (wool colour, sheared, baby). Returns how many it converted.
// Called with stateLock held.
func (w *World) convertHiveSheep() (n int) {
	for _, ent := range w.memState.entities {
		if ent.EntityType != "minecraft:sheep" {
			continue
		}
		ent.EntityType = HiveTextEntity
		for _, k := range []uint32{
			protocol.EntityDataKeyColorIndex,
			protocol.EntityDataKeyColorTwoIndex,
			protocol.EntityDataKeyVariant,
			protocol.EntityDataKeyMarkVariant,
		} {
			delete(ent.Metadata, k)
		}
		if _, ok := ent.Metadata[protocol.EntityDataKeyFlags]; ok {
			ent.Metadata.UnsetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagSheared)
			ent.Metadata.UnsetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagBaby)
		}
		ent.HasMoved = false
		n++
	}
	return n
}
