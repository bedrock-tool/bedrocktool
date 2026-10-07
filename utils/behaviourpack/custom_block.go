package behaviourpack

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type description struct {
	Identifier   string           `json:"identifier"`
	MenuCategory menuCategory     `json:"menu_category,omitempty"`
	Traits       map[string]Trait `json:"traits,omitempty"`
	States       map[string]any   `json:"states,omitempty"`
}

type menuCategory struct {
	Category string `json:"category"`
	Group    string `json:"group,omitempty"`
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

	case "minecraft:collision_box":
		if enabled, ok := value["enabled"].(uint8); ok && enabled == 0 {
			return name, false
		}

		boxes, _ := value["boxes"].([]any)
		if len(boxes) == 0 {
			return name, true
		}

		var boxesData []map[string]any
		for _, box := range boxes {
			box := box.(map[string]any)
			maxX, _ := box["maxX"].(float32)
			maxY, _ := box["maxY"].(float32)
			maxZ, _ := box["maxZ"].(float32)
			minX, _ := box["minX"].(float32)
			minY, _ := box["minY"].(float32)
			minZ, _ := box["minZ"].(float32)
			boxesData = append(boxesData, map[string]any{
				"origin": []any{minX - 8, minY, minZ - 8},
				"size":   []any{maxX - minX, maxY - minY, maxZ - minZ},
			})
		}
		if len(boxesData) == 1 {
			return name, boxesData[0]
		}
		return name, boxesData

	case "minecraft:selection_box":
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
			return name, false
		}
		return name, map[string]any{
			"seconds_to_destroy": value["value"],
		}

	case "minecraft:support":
		return name, value

	case "minecraft:redstone_producer":
		return name, map[string]any{
			"power":                 value["power"],
			"strongly_powered_face": value["strongly_powered_face"],
			"transform_relative":    value["transform_relative"] == uint8(1),
		}

	case "minecraft:destruction_particles":
		return name, value

	case "minecraft:precipitation_interactions":
		return name, value

	case "minecraft:redstone_conductivity":
		redstoneConductor, _ := value["redstoneConductor"].(uint8)
		return name, map[string]any{
			"redstone_conductor": redstoneConductor == 1,
		}

	case "minecraft:random_offset":
		return name, value

	case "minecraft:placement_filter":
		conditions := value["conditions"].([]any)
		var conditionsOut []any
		for _, cond := range conditions {
			cond := cond.(map[string]any)
			allowedFaces := cond["allowed_faces"].(uint8)
			var allowedFacesArr []string

			// TODO others
			if allowedFaces&2 != 0 {
				allowedFacesArr = append(allowedFacesArr, "up")
			}

			var blockFilterOut []any
			for _, filter := range cond["block_filter"].([]any) {
				filter := filter.(map[string]any)
				name, _ := filter["name"].(string)
				if name != "" && len(filter) == 1 {
					blockFilterOut = append(blockFilterOut, name)
					continue
				}
				var filterOut = make(map[string]any)
				if name != "" {
					filterOut["name"] = name
				}
				if states, ok := filter["states"].([]any); ok {
					var statesOut = make(map[string]any)
					for _, state := range states {
						state := state.(map[string]any)
						statesOut[state["state"].(string)] = state["value"]
					}
					filterOut["states"] = statesOut
				}
				if tags, ok := filter["tags"]; ok {
					filterOut["tags"] = tags
				}
				blockFilterOut = append(blockFilterOut, filterOut)
			}

			conditionsOut = append(conditionsOut, map[string]any{
				"allowed_faces": allowedFacesArr,
				"block_filter":  blockFilterOut,
			})
		}
		return name, map[string]any{
			"conditions": conditionsOut,
		}

	case "minecraft:liquid_detection":
		var detectionRules []any
		for _, rule := range value["detectionRules"].([]any) {
			rule := rule.(map[string]any)
			canContainLiquid := rule["canContainLiquid"].(uint8)
			onLiquidTouches := rule["onLiquidTouches"].(string)
			liquidType := rule["liquidType"].(string)
			detectionRules = append(detectionRules, map[string]any{
				"liquid_type":        liquidType,
				"can_contain_liquid": canContainLiquid == 1,
				"on_liquid_touches":  onLiquidTouches,
			})
		}
		return name, map[string]any{
			"detection_rules": detectionRules,
		}

	case "minecraft:leashable":
		return name, value

	case "minecraft:flower_pottable":
		return name, map[string]any{}

	case "minecraft:connection_rule":
		return name, value

	case "minecraft:crafting_table":
		return name, map[string]any{
			"crafting_tags": value["crafting_tags"],
			"table_name":    value["table_name"],
		}

	case "minecraft:display_name":
		return name, value["value"]

	case "minecraft:item_visual":
		materialInstancesDescription, _ := value["materialInstancesDescription"].(map[string]any)
		geometryDescription, _ := value["geometryDescription"].(map[string]any)
		return name, map[string]any{
			"material_instances": processMaterialInstances(materialInstancesDescription, version),
			"geometry":           geometryDescription["identifier"].(string),
		}

	case "minecraft:embedded_visual":
		materials, _ := value["material_instances"].(map[string]any)
		geometry, _ := value["geometry"].(map[string]any)
		return name, map[string]any{
			"material_instances": processMaterials(materials, version),
			"geometry":           geometry["identifier"].(string),
		}

	default:
		if utils.IsDebug() {
			fmt.Printf("unhandled component %s\n%v\n\n", name, value)
		}
	}

	if v, ok := value["value"]; ok {
		return name, v
	}

	return name, value
}

func processMaterials(materials map[string]any, version *string) map[string]any {
	for _, material := range materials {
		material := material.(map[string]any)

		delete(material, "packed_bools")
		if material["culling_shape"] == "" {
			delete(material, "culling_shape")
		}
		if faceDimming, ok := material["face_dimming"].(uint8); ok {
			material["face_dimming"] = faceDimming == 1
		}
		if isotropic, ok := material["isotropic"].(uint8); ok {
			material["isotropic"] = isotropic == 1
			if *version < "1.21.70" {
				*version = "1.21.70"
			}
		}
		if alphaMaskedTint, ok := material["alpha_masked_tint"].(uint8); ok {
			material["alpha_masked_tint"] = alphaMaskedTint == 1
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

func processMaterialInstances(materialInstances map[string]any, version *string) map[string]any {
	if mappings, ok := materialInstances["mappings"].(map[string]any); ok {
		if len(mappings) == 0 {
			delete(materialInstances, "mappings")
		}
	}
	if materials, ok := materialInstances["materials"].(map[string]any); ok {
		return processMaterials(materials, version)
	}
	return materialInstances
}

func processTraits(traits []any) map[string]Trait {
	var out = make(map[string]Trait)
	for _, traitIn := range traits {
		traitIn := traitIn.(map[string]any)
		traitOut := Trait{}
		traitName := traitIn["name"].(string)
		if !strings.ContainsRune(traitName, ':') {
			traitName = "minecraft:" + traitName
		}

		if traitName == "minecraft:connection" {
			traitOut["enabled_states"] = []string{"minecraft:cardinal_connections"}
			out[traitName] = traitOut
			continue
		}

		// enabled states to list of states
		var enabledStatesOut []string
		if enabledStates, ok := traitIn["enabled_states"].(map[string]any); ok {
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

		if !(slices.Contains(enabledStatesOut, "minecraft:facing_direction") || slices.Contains(enabledStatesOut, "minecraft:cardinal_direction")) {
			delete(traitOut, "y_rotation_offset")
		}

		if blocksToCornerWith, ok := traitOut["blocks_to_corner_with"].([]any); ok && len(blocksToCornerWith) == 0 {
			delete(traitOut, "blocks_to_corner_with")
		}

		out[traitName] = traitOut
	}
	return out
}

func processStates(properties []any) map[string]any {
	states := make(map[string]any)
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
			states[propertyName] = enumOut
		}
	}
	return states
}

func parseBlock(block protocol.BlockEntry) (MinecraftBlock, string) {
	version := "1.26.30"
	entry := MinecraftBlock{
		Description: description{
			Identifier: block.Name,
		},
	}

	if traits, ok := block.Properties["traits"].([]any); ok {
		entry.Description.Traits = processTraits(traits)
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
		entry.Description.States = processStates(properties)
	}

	if menu_category, ok := block.Properties["menu_category"].(map[string]any); ok {
		entry.Description.MenuCategory = menuCategory{
			Category: menu_category["category"].(string),
			Group:    menu_category["group"].(string),
		}
	}

	/*
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
	*/

	var minecraftTags []string
	if blockTags, ok := block.Properties["blockTags"].([]any); ok {
		for _, blockTag := range blockTags {
			blockTag := blockTag.(string)
			minecraftTags = append(minecraftTags, blockTag)
		}
	}
	if len(minecraftTags) > 0 {
		if entry.Components == nil {
			entry.Components = make(map[string]any)
		}
		entry.Components["minecraft:tags"] = minecraftTags
	}

	return entry, version
}
