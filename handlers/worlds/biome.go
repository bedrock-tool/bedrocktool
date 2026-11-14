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
	return float64(c.biome.Temperature)
}

func (c *customBiome) Rainfall() float64 {
	return float64(c.biome.Downfall)
}

func (c *customBiome) Depth() float64 {
	return float64(c.biome.Depth)
}

func (c *customBiome) Scale() float64 {
	return float64(c.biome.Scale)
}

// WaterColour is the map water colour the server gave (ARGB).
func (c *customBiome) WaterColour() color.RGBA {
	v := uint32(c.biome.MapWaterColour)
	if v == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func (c *customBiome) Tags() []string {
	return nil
}

func (c *customBiome) String() string {
	return c.name
}

// EncodeBiome is the ID the server gave the biome, which its chunks use.
func (c *customBiome) EncodeBiome() int {
	return int(c.biome.BiomeID)
}
