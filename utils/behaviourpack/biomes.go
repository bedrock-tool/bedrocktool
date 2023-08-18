package behaviourpack

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

func (b *BehaviourPack) AddBiomes(biomesMap map[string]any) {
	for name, biome := range biomesMap {
		_ = biome
		b.biomes = append(b.biomes, biomeBehaviour{
			FormatVersion: "1.13.0",
			MinecraftBiome: MinecraftBiome{
				Description: biomeDescription{
					Identifier: name,
				},
			},
		})
	}

}
