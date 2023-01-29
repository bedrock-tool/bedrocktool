package behaviourpack

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type clientEntityDescription struct {
	Identifier   string `json:"identifier"`
	Spawnable    bool   `json:"is_spawnable"`
	Summonable   bool   `json:"is_summonable"`
	Experimental bool   `json:"is_experimental"`
}

type minecraftClientEntity struct {
	Description     clientEntityDescription `json:"description"`
	ComponentGroups map[string]any          `json:"component_groups"`
	Components      map[string]any          `json:"components"`
	Events          map[string]any          `json:"events"`
}

type entityBehaviour struct {
	FormatVersion   string                `json:"format_version"`
	MinecraftEntity minecraftClientEntity `json:"minecraft:client_entity"`
}

type EntityIn struct {
	Identifier string
	Attr       []protocol.AttributeValue
}

func (bp *BehaviourPack) AddEntity(entity EntityIn) {
	ns, _ := ns_name_split(entity.Identifier)
	if ns == "minecraft" {
		return
	}

	if _, ok := bp.entities[entity.Identifier]; ok {
		return
	}

	entry := entityBehaviour{
		FormatVersion: bp.formatVersion,
		MinecraftEntity: minecraftClientEntity{
			Description: clientEntityDescription{
				Identifier:   entity.Identifier,
				Spawnable:    true,
				Summonable:   true,
				Experimental: true,
			},
			ComponentGroups: nil,
			Components:      make(map[string]any),
			Events:          nil,
		},
	}
	for _, av := range entity.Attr {
		switch av.Name {
		case "minecraft:health":
			entry.MinecraftEntity.Components["minecraft:health"] = map[string]int{
				"value": int(av.Value),
				"max":   int(av.Max),
			}
		case "minecraft:movement":
			entry.MinecraftEntity.Components["minecraft:movement"] = map[string]any{
				"value": av.Value,
			}
		}
	}

	bp.entities[entity.Identifier] = entry
}
