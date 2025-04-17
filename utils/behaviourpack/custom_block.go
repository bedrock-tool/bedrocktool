package behaviourpack

import (
	"fmt"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
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
		return name, processMaterialInstances(value, version)

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
		if *version < "1.19.60" {
			*version = "1.19.60"
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

	default:
		if updater.Version == "" {
			fmt.Printf("unhandled component %s\n%v\n\n", name, value)
		}
	}

	if v, ok := value["value"]; ok {
		return name, v
	}

	return name, value
}

func processMaterialInstances(materialInstances map[string]any, version *string) map[string]any {
	if mappings, ok := materialInstances["mappings"].(map[string]any); ok {
		if len(mappings) == 0 {
			delete(materialInstances, "mappings")
		}
	}
	if materials, ok := materialInstances["materials"].(map[string]any); ok {
		for _, material := range materials {
			material := material.(map[string]any)
			ambientOcclusion, ok := material["ambient_occlusion"].(uint8)
			if ok {
				material["ambient_occlusion"] = ambientOcclusion == 1
			}
			faceDimming, ok := material["face_dimming"].(uint8)
			if ok {
				material["face_dimming"] = faceDimming == 1
			}

			isotropic, ok := material["isotropic"].(uint8)
			if ok {
				material["isotropic"] = isotropic == 1
				if *version < "1.21.70" {
					*version = "1.21.70"
				}
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
	return materialInstances
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
			traitName := traitIn["name"].(string)
			if !strings.ContainsRune(traitName, ':') {
				traitName = "minecraft:" + traitName
			}

			// enabled states to list of states
			enabledStates, ok := traitIn["enabled_states"].(map[string]any)
			if ok {
				var enabledStatesOut []string
				for stateName, stateEnabled := range enabledStates {
					stateEnabled := stateEnabled.(uint8)
					if !strings.ContainsRune(stateName, ':') {
						stateName = "minecraft:" + stateName
					}
					if stateEnabled == 1 {
						enabledStatesOut = append(enabledStatesOut, stateName)
					}
				}
				traitOut["enabled_states"] = enabledStatesOut
			}

			// copy other map values
			for k, v := range traitIn {
				if k == "name" {
					continue
				}
				if k == "enabled_states" {
					continue
				}
				traitOut[k] = v
			}
			entry.Description.Traits[traitName] = traitOut
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

			components := v["components"].(map[string]any)
			for componentName, component := range components {
				component := component.(map[string]any)
				name, value := processComponent(componentName, component, &version)
				if name == "" {
					continue
				}
				perm.Components[name] = value
			}
			entry.Permutations = append(entry.Permutations, perm)
		}
	}

	if components, ok := block.Properties["components"].(map[string]any); ok {
		entry.Components = make(map[string]any)
		for componentName, component := range components {
			component, ok := component.(map[string]any)
			if !ok {
				logrus.Warnf("invalid block component %s %s", block.Name, componentName)
				continue
			}
			name, value := processComponent(componentName, component, &version)
			if name == "" {
				continue
			}
			entry.Components[name] = value
		}
	}

	if properties, ok := block.Properties["properties"].([]any); ok {
		entry.Description.States = make(map[string]any)
		for _, property := range properties {
			property := property.(map[string]any)
			propertyName := property["name"].(string)
			var enumOut []any
			switch enum := property["enum"].(type) {
			case []any:
				enumOut = enum
			case []uint8:
				for _, v := range enum {
					enumOut = append(enumOut, v != 0)
				}
			case []int32:
				for _, v := range enum {
					enumOut = append(enumOut, v)
				}
			default:
				panic("unknown enum encoding")
			}
			if len(enumOut) > 0 {
				entry.Description.States[propertyName] = enumOut
			}
		}
	}

	if menu_category, ok := block.Properties["menu_category"].(map[string]any); ok {
		entry.Description.MenuCategory = menuCategory{
			Category: menu_category["category"].(string),
			Group:    menu_category["group"].(string),
		}
	}

	if properties, ok := block.Properties["properties"].([]any); ok {
		entry.Description.Properties = make(map[string]any)
		for _, property := range properties {
			property := property.(map[string]any)
			propertyName := property["name"].(string)
			switch value := property["enum"].(type) {
			case []int32:
				entry.Description.Properties[propertyName] = value
			case []bool:
				entry.Description.Properties[propertyName] = value
			case []any:
				entry.Description.Properties[propertyName] = value
			}
		}
	}

	return entry, version
}
