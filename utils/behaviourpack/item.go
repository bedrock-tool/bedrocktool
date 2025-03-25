package behaviourpack

import (
	"fmt"

	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type itemDescription struct {
	Category       string `json:"category"`
	Identifier     string `json:"identifier"`
	IsExperimental bool   `json:"is_experimental"`
}

type minecraftItem struct {
	Description itemDescription `json:"description"`
	Components  map[string]any  `json:"components,omitempty"`
}

type itemBehaviour struct {
	FormatVersion string        `json:"format_version"`
	MinecraftItem minecraftItem `json:"minecraft:item"`
}

func (bp *Pack) AddItem(item protocol.ItemEntry) {
	ns, _ := splitNamespace(item.Name)
	if ns == "minecraft" {
		return
	}

	bp.items[item.Name] = &itemBehaviour{
		FormatVersion: "1.20.50",
		MinecraftItem: minecraftItem{
			Description: itemDescription{
				Identifier:     item.Name,
				IsExperimental: true,
			},
			Components: make(map[string]any),
		},
	}
}

func processItemComponent(name string, component map[string]any, componentsOut map[string]any) (string, any) {
	switch name {
	case "item_properties":
		if icon, ok := component["minecraft:icon"].(map[string]any); ok {
			if textures, ok := icon["textures"].(map[string]any); ok {
				componentsOut["minecraft:icon"] = map[string]any{
					"texture": textures["default"],
				}
			}
		}
		return name, component

	case "minecraft:icon":
		if textures, ok := component["textures"].(map[string]any); ok {
			return name, map[string]any{
				"texture": textures["default"],
			}
		}
		return "", nil

	case "minecraft:interact_button":
		return name, component["interact_text"]

	case "item_tags":
		return "", nil

	case "minecraft:durability":
		return name, component

	default:
		if updater.Version == "" {
			fmt.Printf("unhandled component %s\n%v\n\n", name, component)
		}
		return name, component
	}
}

func (bp *Pack) ApplyComponentEntries(entries []protocol.ItemEntry) {
	for _, ice := range entries {
		item, ok := bp.items[ice.Name]
		if !ok {
			continue
		}
		if components, ok := ice.Data["components"].(map[string]any); ok {
			var componentsOut = make(map[string]any)
			for name, component := range components {
				componentMap, ok := component.(map[string]any)
				if !ok {
					fmt.Printf("skipped component %s %v\n", name, component)
					continue
				}
				nameOut, value := processItemComponent(name, componentMap, componentsOut)
				if name == "" {
					continue
				}
				componentsOut[nameOut] = value
			}
			item.MinecraftItem.Components = componentsOut
		}
	}
}
