package behaviourpack

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type description struct {
	Identifier             string         `json:"identifier"`
	IsExperimental         bool           `json:"is_experimental"`
	RegisterToCreativeMenu bool           `json:"register_to_creative_menu"`
	Properties             map[string]any `json:"properties,omitempty"`
	MenuCategory           menuCategory   `json:"menu_category,omitempty"`
}

type menuCategory struct {
	Category string `json:"category"`
	Group    string `json:"group"`
}

func menu_category_from_map(in map[string]any) menuCategory {
	return menuCategory{
		Category: in["category"].(string),
		Group:    in["group"].(string),
	}
}

type permutation struct {
	Components map[string]any `json:"components"`
	Condition  string         `json:"condition"`
}

func permutation_from_map(in map[string]any) permutation {
	return permutation{
		Components: in["components"].(map[string]any),
		Condition:  in["condition"].(string),
	}
}

type MinecraftBlock struct {
	Description  description    `json:"description"`
	Components   map[string]any `json:"components,omitempty"`
	Permutations []permutation  `json:"permutations,omitempty"`
}

func parseBlock(block protocol.BlockEntry) MinecraftBlock {
	entry := MinecraftBlock{
		Description: description{
			Identifier:             block.Name,
			IsExperimental:         true,
			RegisterToCreativeMenu: true,
		},
	}

	if perms, ok := block.Properties["permutations"].([]any); ok {
		for _, v := range perms {
			perm := permutation_from_map(v.(map[string]any))
			if v, ok := perm.Components["minecraft:transformation"].(map[string]any); ok {
				// rotation
				rx, _ := v["RX"].(int32)
				ry, _ := v["RY"].(int32)
				rz, _ := v["RZ"].(int32)

				// scale
				sx, _ := v["SX"].(float32)
				sy, _ := v["SY"].(float32)
				sz, _ := v["SZ"].(float32)

				// translation
				tx, _ := v["TX"].(float32)
				ty, _ := v["TY"].(float32)
				tz, _ := v["TZ"].(float32)

				perm.Components["minecraft:transformation"] = map[string][]float32{
					"translation": {tx, ty, tz},
					"scale":       {sx, sy, sz},
					"rotation":    {float32(rx) * 90, float32(ry) * 90, float32(rz) * 90},
				}
			}
			if v, ok := perm.Components["minecraft:geometry"].(map[string]any); ok {
				if identifier, ok := v["identifier"].(string); ok {
					perm.Components["minecraft:geometry"] = identifier
				}
			}
			entry.Permutations = append(entry.Permutations, perm)
		}
	}

	if comps, ok := block.Properties["components"].(map[string]any); ok {
		delete(comps, "minecraft:creative_category")

		for k, v := range comps {
			if v, ok := v.(map[string]any); ok {
				// fix {"value": 0.1} -> 0.1
				if v, ok := v["value"]; ok {
					comps[k] = v
				}
				// fix {"lightLevel": 15} -> 15
				if v, ok := v["lightLevel"]; ok {
					comps[k] = v
				}

				// fix missing * instance
				if k == "minecraft:material_instances" {
					if m, ok := v["materials"].(map[string]any); ok {
						comps[k] = m
					}
				}
				// fix {"identifier": "name"} -> "name"
				if v, ok := v["identifier"]; ok {
					comps[k] = v
				}
				// fix {"emission": "name"} -> "name"
				if v, ok := v["emission"]; ok {
					comps[k] = v
				}

				if v, ok := v["triggerType"]; ok {
					comps[k] = map[string]any{
						"event": v.(string),
					}
				}
			}
		}

		if friction, ok := comps["minecraft:friction"].(float32); ok {
			if friction == 0.4 {
				delete(comps, "minecraft:friction")
			}
		}

		entry.Components = comps
	}

	if menu, ok := block.Properties["menu_category"].(map[string]any); ok {
		entry.Description.MenuCategory = menu_category_from_map(menu)
	}

	if props, ok := block.Properties["properties"].([]any); ok {
		entry.Description.Properties = make(map[string]any)
		for _, v := range props {
			v := v.(map[string]any)
			name := v["name"].(string)
			switch a := v["enum"].(type) {
			case []int32:
				entry.Description.Properties[name] = a
			case []bool:
				entry.Description.Properties[name] = a
			case []any:
				entry.Description.Properties[name] = a
			}
		}
	}

	return entry
}
