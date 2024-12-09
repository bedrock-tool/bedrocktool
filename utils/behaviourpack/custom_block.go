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

type Trait map[string]any

type MinecraftBlock struct {
	Description  description    `json:"description"`
	Components   map[string]any `json:"components,omitempty"`
	Permutations []permutation  `json:"permutations,omitempty"`
}

func processComponent(name string, value map[string]any, version *string) (string, any) {
	switch name {
	case "minecraft:block_light_filter", "minecraft:light_dampening":
		lightLevel, ok := value["lightLevel"]
		if !ok || lightLevel == float32(-1) {
			return "", nil
		}
		return "minecraft:light_dampening", lightLevel

	case "minecraft:material_instances":
		return name, processMaterialInstances(value)

	case "minecraft:geometry":
		return name, value["identifier"].(string)

	case "minecraft:light_emission":
		return name, value["emission"]

	case "minecraft:friction":
		if friction, ok := value["value"].(float32); ok {
			if friction == 0.4 {
				return "", nil
			}
			return name, friction
		}

	case "minecraft:transformation":
		// rotation
		rx, _ := value["RX"].(int32)
		ry, _ := value["RY"].(int32)
		rz, _ := value["RZ"].(int32)

		// scale
		sx, _ := value["SX"].(float32)
		sy, _ := value["SY"].(float32)
		sz, _ := value["SZ"].(float32)

		// translation
		tx, _ := value["TX"].(float32)
		ty, _ := value["TY"].(float32)
		tz, _ := value["TZ"].(float32)

		return name, map[string][]float32{
			"translation": {tx, ty, tz},
			"scale":       {sx, sy, sz},
			"rotation":    {float32(rx) * 90, float32(ry) * 90, float32(rz) * 90},
		}

	case "minecraft:collision_box", "minecraft:selection_box":
		if enabled, ok := value["enabled"].(uint8); ok && enabled == 0 {
			return name, false
		}
		return name, map[string]any{
			"origin": value["origin"],
			"size":   value["size"],
		}

	case "minecraft:on_player_placing":
		return "", nil

	case "minecraft:custom_components", "minecraft:creative_category":
		return "", nil

	case "minecraft:destructible_by_mining":
		if value["value"] == float32(-1) {
			return "", nil
		}
	}

	if v, ok := value["value"]; ok {
		return name, v
	}

	return name, value
}

func processMaterialInstances(material_instances map[string]any) map[string]any {
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

func parseBlock(block protocol.BlockEntry) (MinecraftBlock, string) {
	version := "1.21.0"
	entry := MinecraftBlock{
		Description: description{
			Identifier:             block.Name,
			IsExperimental:         true,
			RegisterToCreativeMenu: true,
		},
	}

	if traits, ok := block.Properties["traits"].([]any); ok {
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

	if permutations, ok := block.Properties["permutations"].([]any); ok {
		if version < "1.19.70" {
			version = "1.19.70"
		}

		for _, v := range permutations {
			v := v.(map[string]any)
			perm := permutation{
				Components: make(map[string]any),
				Condition:  v["condition"].(string),
			}

			if strings.Contains(perm.Condition, "query.block_property") && version > "1.19.80" {
				version = "1.19.80"
			}

			comps := v["components"].(map[string]any)
			for k, v := range comps {
				name, value := processComponent(k, v.(map[string]any), &version)
				if name == "" {
					continue
				}
				perm.Components[name] = value
			}

			entry.Permutations = append(entry.Permutations, perm)
		}
	}

	if components, ok := block.Properties["components"].(map[string]any); ok {
		comps := make(map[string]any)
		for k, v := range components {
			name, value := processComponent(k, v.(map[string]any), &version)
			if name == "" {
				continue
			}
			if name == "minecraft:selection_box" && version < "1.19.60" {
				version = "1.19.60"
			}
			comps[name] = value
		}
		entry.Components = comps
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

			if len(enum2) > 0 {
				entry.Description.States[name] = enum2
			}
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
