package behaviourpack

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type EntityDescription struct {
	Identifier   string `json:"identifier"`
	Spawnable    bool   `json:"is_spawnable"`
	Summonable   bool   `json:"is_summonable"`
	Experimental bool   `json:"is_experimental"`
}

type MinecraftEntity struct {
	Description     EntityDescription `json:"description"`
	ComponentGroups map[string]any    `json:"component_groups"`
	Components      map[string]any    `json:"components"`
	Events          map[string]any    `json:"events,omitempty"`
}

type entityBehaviour struct {
	FormatVersion   string          `json:"format_version"`
	MinecraftEntity MinecraftEntity `json:"minecraft:entity"`
}

type EntityIn struct {
	Identifier string
	Attr       []protocol.AttributeValue
	Meta       protocol.EntityMetadata
}

func (bp *BehaviourPack) AddEntity(entity EntityIn) {
	ns, _ := ns_name_split(entity.Identifier)
	if ns == "minecraft" {
		return
	}

	entry, ok := bp.entities[entity.Identifier]
	if !ok {
		entry = &entityBehaviour{
			FormatVersion: bp.formatVersion,
			MinecraftEntity: MinecraftEntity{
				Description: EntityDescription{
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

	bp.entities[entity.Identifier] = entry
}
