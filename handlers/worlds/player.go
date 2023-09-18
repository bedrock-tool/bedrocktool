package worlds

import (
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/df-mc/dragonfly/server/item/inventory"
)

func (w *worldsHandler) playerData() (ret map[string]any) {
	ret = map[string]any{
		"format_version": "1.12.0",
		"identifier":     "minecraft:player",
	}

	if len(w.serverState.playerInventory) > 0 && w.settings.SaveInventories {
		inv := inventory.New(len(w.serverState.playerInventory), nil)
		for i, ii := range w.serverState.playerInventory {
			inv.SetItem(i, utils.StackToItem(ii.Stack))
		}
		ret["Inventory"] = nbtconv.InvToNBT(inv)
	}

	ret["abilities"] = map[string]any{
		"doorsandswitches":       true,
		"op":                     true,
		"opencontainers":         true,
		"teleport":               true,
		"attackmobs":             true,
		"instabuild":             true,
		"permissionsLevel":       int32(3),
		"flying":                 false,
		"lightning":              false,
		"playerPermissionsLevel": int32(2),
		"attackplayers":          true,
		"build":                  true,
		"flySpeed":               float32(0.05),
		"invulnerable":           true,
		"mayfly":                 true,
		"mine":                   true,
		"walkSpeed":              float32(0.1),
	}

	type attribute struct {
		Name       string
		Base       float32
		Current    float32
		DefaultMax float32
		DefaultMin float32
		Max        float32
		Min        float32
	}

	ret["Attributes"] = []attribute{
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1024,
			DefaultMin: -1024,
			Max:        1024,
			Min:        -1024,
			Name:       "minecraft:luck",
		},
		{
			Base:       20,
			Current:    20,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:health",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 16,
			DefaultMin: 0,
			Max:        16,
			Min:        0,
			Name:       "minecraft:absorption",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1,
			DefaultMin: 0,
			Max:        1,
			Min:        0,
			Name:       "minecraft:knockback_resistance",
		},
		{
			Base:       0.1,
			Current:    0.1,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:movement",
		},
		{
			Base:       0.02,
			Current:    0.02,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:underwater_movement",
		},
		{
			Base:       0.02,
			Current:    0.02,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:lava_movement",
		},
		{
			Base:       16,
			Current:    16,
			DefaultMax: 2048,
			DefaultMin: 0,
			Max:        2048,
			Min:        0,
			Name:       "minecraft:follow_range",
		},
		{
			Base:       1,
			Current:    1,
			DefaultMax: 1,
			DefaultMin: 1,
			Max:        1,
			Min:        1,
			Name:       "minecraft:attack_damage",
		},
		{
			Base:       20,
			Current:    20,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.hunger",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.exhaustion",
		},
		{
			Base:       5,
			Current:    5,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.saturation",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 24791,
			DefaultMin: 0,
			Max:        24791,
			Min:        0,
			Name:       "minecraft:player.level",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1,
			DefaultMin: 0,
			Max:        1,
			Min:        0,
			Name:       "minecraft:player.experience",
		},
	}

	ret["Tags"] = []string{}
	ret["OnGround"] = true

	spawn := w.proxy.Player.Position

	ret["SpawnX"] = int32(spawn.X())
	ret["SpawnY"] = int32(spawn.Y())
	ret["SpawnZ"] = int32(spawn.Z())

	ret["Pos"] = []float32{
		float32(spawn.X()),
		float32(spawn.Y()),
		float32(spawn.Z()),
	}

	ret["Rotation"] = []float32{
		w.proxy.Player.Pitch,
		w.proxy.Player.Yaw,
	}

	return
}
