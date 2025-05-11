package behaviourpack

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type biomeBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBiome MinecraftBiome `json:"minecraft:biome"`
}

type biomeDescription struct {
	Identifier string `json:"identifier"`
}

type MinecraftBiome struct {
	Description biomeDescription `json:"description"`
}

func (b *Pack) AddBiome(biomeName string, definition protocol.BiomeDefinition) {
	_ = definition
	b.biomes = append(b.biomes, biomeBehaviour{
		FormatVersion: "1.13.0",
		MinecraftBiome: MinecraftBiome{
			Description: biomeDescription{
				Identifier: biomeName,
			},
		},
	})
}
