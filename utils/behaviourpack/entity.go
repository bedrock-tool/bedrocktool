package behaviourpack

import (
	"fmt"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type EntityDescription struct {
	Identifier   string `json:"identifier"`
	Spawnable    bool   `json:"is_spawnable"`
	Summonable   bool   `json:"is_summonable"`
	Experimental bool   `json:"is_experimental"`

	Properties map[string]EntityPropertyJson `json:"properties,omitempty"`
}

type MinecraftEntity struct {
	Description     *EntityDescription `json:"description"`
	ComponentGroups map[string]any     `json:"component_groups,omitempty"`
	Components      map[string]any     `json:"components,omitempty"`
	Events          map[string]any     `json:"events,omitempty"`
}

type entityBehaviour struct {
	FormatVersion   string           `json:"format_version"`
	MinecraftEntity *MinecraftEntity `json:"minecraft:entity"`
}

type EntityIn struct {
	Identifier string
	Attr       []protocol.AttributeValue
	Meta       protocol.EntityMetadata
}

type EntityProperty struct {
	Type int32
	Name string
	Min  float32
	Max  float32
	Enum []any
}

type EntityPropertyJson struct {
	Type       string `json:"type"`
	Values     any    `json:"values,omitempty"`
	Range      []any  `json:"range,omitempty"`
	Default    any    `json:"default,omitempty"`
	ClientSync bool   `json:"client_sync"`
}

func (bp *Pack) AddEntity(entity EntityIn) {
	ns, _ := ns_name_split(entity.Identifier)
	if ns == "minecraft" {
		return
	}

	entry, ok := bp.entities[entity.Identifier]
	if !ok {
		entry = &entityBehaviour{
			FormatVersion: bp.formatVersion,
			MinecraftEntity: &MinecraftEntity{
				Description: &EntityDescription{
					Identifier:   entity.Identifier,
					Spawnable:    true,
					Summonable:   true,
					Experimental: true,
				},
				ComponentGroups: make(map[string]any),
				Components:      make(map[string]any),
				Events:          nil,
			},
		}
	}

	for _, av := range entity.Attr {
		m := map[string]int{
			"value": int(av.Value),
			"min":   int(av.Min),
		}

		if av.Max > 0 && av.Max < 0xffffff {
			m["max"] = int(av.Max)
		}

		entry.MinecraftEntity.Components[av.Name] = m
	}

	if scale, ok := entity.Meta[protocol.EntityDataKeyScale].(float32); ok {
		entry.MinecraftEntity.Components["minecraft:scale"] = map[string]any{
			"value": scale,
		}
	}

	width, widthOk := entity.Meta[protocol.EntityDataKeyWidth].(float32)
	height, heightOk := entity.Meta[protocol.EntityDataKeyHeight].(float32)
	if widthOk || heightOk {
		entry.MinecraftEntity.Components["minecraft:collision_box"] = map[string]any{
			"width":  width,
			"height": height,
		}
	}

	if _, ok := entity.Meta[protocol.EntityDataKeyFlags]; ok {
		AlwaysShowName := entity.Meta.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagAlwaysShowName)
		if AlwaysShowName {
			entry.MinecraftEntity.Components["minecraft:nameable"] = map[string]any{
				"always_show": true,
			}
		}
	}

	entry.MinecraftEntity.Components["minecraft:pushable"] = map[string]any{
		"is_pushable":           false,
		"is_pushable_by_piston": false,
	}
	entry.MinecraftEntity.Components["minecraft:damage_sensor"] = map[string]any{
		"triggers": map[string]any{
			"deals_damage": false,
		},
	}
	entry.MinecraftEntity.Components["minecraft:is_stackable"] = map[string]any{}
	entry.MinecraftEntity.Components["minecraft:push_through"] = 1

	props, ok := bp.entityProperties[entity.Identifier]
	if ok {
		properties := make(map[string]EntityPropertyJson)
		for _, v := range props {
			var prop EntityPropertyJson
			prop.ClientSync = true
			switch v.Type {
			case propertyTypeInt:
				prop.Type = "int"
				prop.Range = []any{int(v.Min), int(v.Max)}
				prop.Default = int(v.Min)
			case propertyTypeFloat:
				prop.Type = "float"
				prop.Range = []any{v.Min, v.Max}
				prop.Default = v.Min
			case propertyTypeBool:
				prop.Type = "bool"
				prop.Default = false
			case propertyTypeEnum:
				prop.Type = "enum"
				prop.Values = v.Enum
				prop.Default = v.Enum[0]
			}
			properties[v.Name] = prop
		}
		entry.MinecraftEntity.Description.Properties = properties
	}

	bp.entities[entity.Identifier] = entry
}

const (
	propertyTypeInt = iota
	propertyTypeFloat
	propertyTypeBool
	propertyTypeEnum
)

func (bp *Pack) GetEntityTypeProperties(entityType string) []EntityProperty {
	return bp.entityProperties[entityType]
}

func (bp *Pack) SyncActorProperty(pk *packet.SyncActorProperty) {
	entityType, ok := pk.PropertyData["type"].(string)
	if !ok {
		return
	}
	properties, ok := pk.PropertyData["properties"].([]any)
	if !ok {
		return
	}

	var propertiesOut = make([]EntityProperty, 0, len(properties))
	for _, property := range properties {
		property := property.(map[string]any)
		propertyName, ok := property["name"].(string)
		if !ok {
			continue
		}
		propertyType, ok := property["type"].(int32)
		if !ok {
			continue
		}

		var prop EntityProperty
		prop.Name = propertyName
		prop.Type = propertyType

		switch propertyType {
		case propertyTypeInt:
			min, ok := property["min"].(int32)
			if !ok {
				continue
			}
			max, ok := property["max"].(int32)
			if !ok {
				continue
			}
			prop.Min = float32(min)
			prop.Max = float32(max)
		case propertyTypeFloat:
			min, ok := property["min"].(int32)
			if !ok {
				continue
			}
			max, ok := property["max"].(int32)
			if !ok {
				continue
			}
			prop.Min = float32(min)
			prop.Max = float32(max)
		case propertyTypeBool:
		case propertyTypeEnum:
			prop.Enum, _ = property["enum"].([]any)
		default:
			fmt.Printf("Unknown property type %d", propertyType)
			continue
		}

		propertiesOut = append(propertiesOut, prop)
	}

	bp.entityProperties[entityType] = propertiesOut
}
