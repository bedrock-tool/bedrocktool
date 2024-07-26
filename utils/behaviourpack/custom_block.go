package behaviourpack

import (
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type description struct {
	Identifier             string           `json:"identifier"`
	IsExperimental         bool             `json:"is_experimental"`
	RegisterToCreativeMenu bool             `json:"register_to_creative_menu"`
	Properties             map[string]any   `json:"properties,omitempty"`
	MenuCategory           menuCategory     `json:"menu_category,omitempty"`
	Traits                 map[string]Trait `json:"traits,omitempty"`
	States                 map[string]any   `json:"states,omitempty"`
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

type Trait map[string]any

type MinecraftBlock struct {
	Description  description    `json:"description"`
	Components   map[string]any `json:"components,omitempty"`
	Permutations []permutation  `json:"permutations,omitempty"`
}

func parseBlock(block protocol.BlockEntry) (MinecraftBlock, string) {
	version := "1.16.0"
	entry := MinecraftBlock{
		Description: description{
			Identifier:             block.Name,
			IsExperimental:         true,
			RegisterToCreativeMenu: true,
		},
	}

	if traits, ok := block.Properties["traits"].([]any); ok {
		version = "1.21.0"
		entry.Description.Traits = make(map[string]Trait)

		for _, traitIn := range traits {
			traitIn := traitIn.(map[string]any)
			traitOut := Trait{}
			name := traitIn["name"].(string)
			if !strings.ContainsRune(name, ':') {
				name = "minecraft:" + name
			}

			for k, v := range traitIn {
				if k == "name" {
					continue
				}
				if k == "enabled_states" {
					var enabled_states []string
					v := v.(map[string]any)
					for name2, v2 := range v {
						v2 := v2.(uint8)
						if !strings.ContainsRune(name2, ':') {
							name2 = "minecraft:" + name2
						}
						if v2 == 1 {
							enabled_states = append(enabled_states, name2)
						}
					}
					traitOut[k] = enabled_states
					continue
				}
				traitOut[k] = v
			}
			entry.Description.Traits[name] = traitOut
		}
	}

	processMaterialInstances := func(material_instances map[string]any) map[string]any {
		if mappings, ok := material_instances["mappings"].(map[string]any); ok {
			if len(mappings) == 0 {
				delete(material_instances, "mappings")
			}
		}
		if materials, ok := material_instances["materials"].(map[string]any); ok {
			for _, material := range materials {
				material := material.(map[string]any)
				ambient_occlusion, ok := material["ambient_occlusion"].(uint8)
				if ok {
					material["ambient_occlusion"] = ambient_occlusion == 1
				}
				face_dimming, ok := material["face_dimming"].(uint8)
				if ok {
					material["face_dimming"] = face_dimming == 1
				}
			}

			if _, ok := materials["*"]; !ok {
				up, ok := materials["up"]
				if ok {
					materials["*"] = up
				} else {
					for _, side := range materials {
						materials["*"] = side
						break
					}
				}
			}

			return materials
		}
		return material_instances
	}

	processComponents := func(components map[string]any) {
		delete(components, "minecraft:creative_category")

		for k, v := range components {
			if v, ok := v.(map[string]any); ok {
				if k == "minecraft:friction" {
					if friction, ok := v["value"].(float32); ok {
						if friction == 0.4 {
							delete(components, "minecraft:friction")
						} else {
							components[k] = friction
						}
					}
					continue
				}

				// fix missing * instance
				if k == "minecraft:material_instances" {
					components[k] = processMaterialInstances(v)
					if m, ok := v["materials"].(map[string]any); ok {
						components[k] = m
					}
					continue
				}

				if k == "minecraft:transformation" {
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

					components[k] = map[string][]float32{
						"translation": {tx, ty, tz},
						"scale":       {sx, sy, sz},
						"rotation":    {float32(rx) * 90, float32(ry) * 90, float32(rz) * 90},
					}
					continue
				}

				if k == "minecraft:geometry" {
					if identifier, ok := v["identifier"].(string); ok {
						components[k] = identifier
					}
					continue
				}

				// fix {"value": 0.1} -> 0.1
				if v, ok := v["value"]; ok {
					components[k] = v
					continue
				}

				// fix {"lightLevel": 15} -> 15
				if v, ok := v["lightLevel"]; ok {
					components[k] = v
					continue
				}

				// fix {"identifier": "name"} -> "name"
				if v, ok := v["identifier"]; ok {
					components[k] = v
					continue
				}

				// fix {"emission": "name"} -> "name"
				if v, ok := v["emission"]; ok {
					components[k] = v
					continue
				}

				if v, ok := v["triggerType"]; ok {
					components[k] = map[string]any{
						"event": v.(string),
					}
					continue
				}

				if k == "minecraft:collision_box" {
					if enabled, ok := v["enabled"].(uint8); ok {
						if enabled == 0 {
							components[k] = false
							continue
						}
					}
				}
			}

			if k == "minecraft:custom_components" {
				delete(components, k)
				continue
			}
		}
	}

	if permutations, ok := block.Properties["permutations"].([]any); ok {
		for _, v := range permutations {
			perm := permutation_from_map(v.(map[string]any))
			processComponents(perm.Components)
			entry.Permutations = append(entry.Permutations, perm)
		}
	}

	if components, ok := block.Properties["components"].(map[string]any); ok {
		processComponents(components)
		entry.Components = components
	}

	if properties, ok := block.Properties["properties"].([]any); ok {
		entry.Description.States = make(map[string]any)
		for _, property := range properties {
			property := property.(map[string]any)
			name, ok := property["name"].(string)
			if !ok {
				continue
			}

			var enum2 []any
			enum, ok := property["enum"].([]any)
			if ok {
				enum2 = enum
			} else {
				enum, ok := property["enum"].([]uint8)
				if ok {
					for _, v := range enum {
						enum2 = append(enum2, v != 0)
					}
				}
			}

			entry.Description.States[name] = enum2
		}
	}

	if menu_category, ok := block.Properties["menu_category"].(map[string]any); ok {
		entry.Description.MenuCategory = menu_category_from_map(menu_category)
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

	return entry, version
}
