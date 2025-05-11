package worlds

import (
	"image/color"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type customBiome struct {
	name  string
	biome protocol.BiomeDefinition
	pk    *packet.BiomeDefinitionList
}

func (c *customBiome) Temperature() float64 {
	return 0.5
}

func (c *customBiome) Rainfall() float64 {
	return 0.5
}

func (c *customBiome) Depth() float64 {
	return 0
}

func (c *customBiome) Scale() float64 {
	return 0
}

func (c *customBiome) WaterColour() color.RGBA {
	return color.RGBA{}
}

func (c *customBiome) Tags() []string {
	return nil
}

func (c *customBiome) String() string {
	return c.name
}

func (c *customBiome) EncodeBiome() int {
	return 0
}
