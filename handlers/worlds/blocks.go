package worlds

import (
	"fmt"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/zaataylor/cartesian/cartesian"
)

var traitLookup = map[string][]any{
	"minecraft:cardinal_direction": {
		"north", "east", "south", "west",
	},
	"minecraft:facing_direction": {
		"north", "east", "south", "west", "down", "up",
	},
	"minecraft:corner_and_cardinal_direction": {
		"inner_left", "inner_right", "outer_left", "outer_right", "none",
	},
	"minecraft:sixteen_way_rotation": {
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16",
	},
	"minecraft:vertical_half": {
		"top", "bottom",
	},
	"minecraft:block_face": {
		"north", "east", "south", "west", "down", "up",
	},
}

func splitNamespace(identifier string) (ns, name string) {
	sp := strings.SplitN(identifier, ":", 2)
	if len(sp) == 1 {
		return "", identifier
	}
	return sp[0], sp[1]
}

// AddCustomBlocks registers every state of the server's custom blocks.
// A state is registered once even when the server lists the same block
// twice, or names a property both in "properties" and through a trait:
// dragonfly panics on a second registration ("cannot register the same
// state twice", #327).
func AddCustomBlocks(reg world.BlockRegistry, entries []protocol.BlockEntry) error {
	registered := make(map[string]struct{})
	for _, entry := range entries {
		ns, _ := splitNamespace(entry.Name)
		if ns == "minecraft" {
			continue
		}

		var propertyNames []string
		var propertyValues []any
		addProperty := func(name string, values any) {
			if slices.Contains(propertyNames, name) {
				return
			}
			propertyNames = append(propertyNames, name)
			propertyValues = append(propertyValues, values)
		}

		props, ok := entry.Properties["properties"].([]any)
		if ok {
			for _, v := range props {
				v := v.(map[string]any)
				name := v["name"].(string)
				addProperty(name, v["enum"])
			}
		}

		traits, ok := entry.Properties["traits"].([]any)
		if ok {
			for _, trait := range traits {
				trait := trait.(map[string]any)
				name := trait["name"].(string)
				switch enabled_states := trait["enabled_states"].(type) {
				case map[string]any:
					for k, enabled := range enabled_states {
						if !strings.ContainsRune(k, ':') {
							k = "minecraft:" + k
						}
						enabled := enabled.(uint8)
						if enabled == 0 {
							continue
						}
						v, ok := traitLookup[k]
						if !ok {
							return fmt.Errorf("unresolved trait %s", k)
						}

						addProperty(k, v)
					}
				case int32:
					if name == "minecraft:connection" {
						var b = []bool{false, true}
						for _, side := range []string{"north", "south", "west", "east"} {
							addProperty("minecraft:connection_"+side, b)
						}
					} else {
						return fmt.Errorf("unresolved trait %s", name)
					}
				}
			}
		}

		permutations := cartesian.NewCartesianProduct(propertyValues).Values()

		for _, values := range permutations {
			m := make(map[string]any)
			for i, value := range values {
				name := propertyNames[i]
				m[name] = value
			}
			// fmt prints map keys sorted, so this names the state exactly
			key := entry.Name + fmt.Sprint(m)
			if _, ok := registered[key]; ok {
				continue
			}
			registered[key] = struct{}{}
			reg.RegisterBlockState(world.BlockState{
				Name:       entry.Name,
				Properties: m,
			})
		}
	}

	return nil
}
