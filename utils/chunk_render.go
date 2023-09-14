package utils

import (
	"image"
	"image/color"
	"strings"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func isBlockLightblocking(b world.Block) bool {
	d, isDiffuser := b.(block.LightDiffuser)
	noDiffuse := isDiffuser && d.LightDiffusionLevel() == 0
	return !noDiffuse
}

var customBlockColors = map[string]color.RGBA64{}

func RGBAToRGBA64(rgba color.RGBA) color.RGBA64 {
	return color.RGBA64{
		R: uint16(rgba.R) * 257,
		G: uint16(rgba.G) * 257,
		B: uint16(rgba.B) * 257,
		A: uint16(rgba.A) * 257,
	}
}

var waterColor color.RGBA64

func init() {
	waterColor = RGBAToRGBA64(block.Water{}.Color())
}

func blockColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) (blockColor color.RGBA64) {
	if y <= int16(c.Range().Min()) {
		return color.RGBA64{0, 0, 0, 0}
	}
	rid := c.Block(x, y, z, 0)
	/*
		if idx, ok := ridToIdx[rid]; ok {
			return color.RGBA64{idx.X, idx.Y, 0, 0xff}
		}
		return color.RGBA64{}
	*/

	blockColor = color.RGBA64{0xffff, 0, 0xffff, 0xffff}
	b, found := world.BlockByRuntimeID(rid)
	if !found {
		return blockColor
	}

	if _, isWater := b.(block.Water); isWater {
		// get the first non water block at the position
		heightBlock := c.HeightMap().At(x, z)
		depth := y - heightBlock
		if depth > 0 {
			blockColor = blockColorAt(c, x, heightBlock, z)
		} else {
			blockColor = color.RGBA64{0, 0, 0, 0}
		}

		// blend that blocks color with water depending on depth
		waterColor.A = min(150+uint16(depth)*7, 230) * 257
		blockColor = BlendColors(blockColor, waterColor)
		blockColor.R -= uint16(depth * 6 * 257)
		blockColor.G -= uint16(depth * 6 * 257)
		blockColor.B -= uint16(depth * 6 * 257)
		return blockColor
	} else {
		if b2, ok := b.(world.UnknownBlock); ok {
			name, _ := b2.EncodeBlock()
			blockColor, ok = customBlockColors[name]
			if !ok {
				if name == "minecraft:monster_egg" {
					name = "minecraft:" + b2.Properties["monster_egg_stone_type"].(string)
				}
				if name == "minecraft:suspicious_sand" {
					name = "minecraft:sand"
				}
				if name == "minecraft:suspicious_gravel" {
					name = "minecraft:gravel"
				}
				if name == "minecraft:pointed_dripstone" {
					name = "minecraft:dripstone_block"
				}
				if name == "minecraft:dark_oak_hanging_sign" {
					name = "minecraft:darkoak_hanging_sign"
				}
				if name == "minecraft:mangrove_hanging_sign" {
					name = "minecraft:mangrove_wood"
				}
				if name == "minecraft:crimson_hanging_sign" {
					name = "minecraft:crimson_fungus"
				}
				if name == "minecraft:warped_standing_sign" {
					name = "minecraft:warped_fungus"
				}
				if name == "minecraft:warped_hanging_sign" {
					name = "minecraft:warped_fungus"
				}
				if name == "minecraft:oak_hanging_sign" {
					name = "minecraft:oak_stairs"
				}
				if strings.HasSuffix(name, "_hanging_sign") {
					name = strings.Replace(name, "_hanging", "_standing", 1)
				}
				if strings.HasSuffix(name, "_candle_cake") {
					name = "minecraft:cake"
				}
				blockColor = RGBAToRGBA64(LookupColor(name))
			}
		} else {
			blockColor = RGBAToRGBA64(b.Color())

		}

		if blockColor.R == 0xffff && blockColor.G == 0x0 && blockColor.B == 0xffff {
			if Version == "" {
				println(b.EncodeBlock())
				b.Color()
			}
		}

		if blockColor.A != 0xffff {
			blockColor = BlendColors(blockColorAt(c, x, y-1, z), blockColor)
		}
		return blockColor
	}
}

func chunkGetColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) color.RGBA64 {
	haveUp := false
	cube.Pos{int(x), int(y), int(z)}.
		Side(cube.FaceUp).
		Neighbours(func(neighbour cube.Pos) {
			if neighbour.X() < 0 || neighbour.X() >= 16 || neighbour.Z() < 0 || neighbour.Z() >= 16 || neighbour.Y() > c.Range().Max() || haveUp {
				return
			}
			blockRid := c.Block(uint8(neighbour[0]), int16(neighbour[1]), uint8(neighbour[2]), 0)
			if blockRid > 0 {
				b, found := world.BlockByRuntimeID(blockRid)
				if found {
					if isBlockLightblocking(b) {
						haveUp = true
					}
				}
			}
		}, cube.Range{int(y + 1), int(y + 1)})

	blockColor := blockColorAt(c, x, y, z)
	if haveUp && (x+z)%2 == 0 {
		if blockColor.R > 10*0xff {
			blockColor.R -= 10 * 0xff
		}
		if blockColor.G > 10*0xff {
			blockColor.G -= 10 * 0xff
		}
		if blockColor.B > 10*0xff {
			blockColor.B -= 10 * 0xff
		}
	}
	return blockColor
}

func Chunk2Img(c *chunk.Chunk) *image.RGBA64 {
	img := image.NewRGBA64(image.Rect(0, 0, 16, 16))
	hm := c.HeightMapWithWater()

	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			img.SetRGBA64(
				int(x), int(z),
				chunkGetColorAt(c, x, hm.At(x, z), z),
			)
		}
	}
	return img
}
