package utils

import (
	"image"
	"image/color"

	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

func isBlockLightblocking(b world.Block) bool {
	d, isDiffuser := b.(block.LightDiffuser)
	noDiffuse := isDiffuser && d.LightDiffusionLevel() == 0
	return !noDiffuse
}

var waterColor = block.Water{}.Color()
var notFoundColor = color.RGBA{0xff, 0, 0xff, 0xff}

func (cr *ChunkRenderer) blockColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) (blockColor color.RGBA) {
	if y <= int16(c.Range().Min()) {
		return color.RGBA{0, 0, 0, 0}
	}
	rid := c.Block(x, y, z, 0)

	br := c.BlockRegistry.(world.BlockRegistry)
	b, found := br.BlockByRuntimeID(rid)
	if !found {
		return notFoundColor
	}

	if _, isWater := b.(block.Water); isWater {
		// get the first non water block at the position
		heightBlock := c.HeightMap().At(x, z)
		depth := y - heightBlock
		if depth > 0 {
			blockColor = cr.blockColorAt(c, x, heightBlock, z)
		} else {
			blockColor = color.RGBA{0, 0, 0, 0}
		}

		// blend that blocks color with water depending on depth
		waterColor.A = uint8(min(150+depth*7, 230))
		blockColor = BlendColors(blockColor, waterColor)
		blockColor.R -= uint8(depth * 6)
		blockColor.G -= uint8(depth * 6)
		blockColor.B -= uint8(depth * 6)
		return blockColor
	}

	if b2, ok := b.(world.UnknownBlock); ok {
		name, _ := b2.EncodeBlock()
		customColor, ok := cr.customBlockColors[name]
		if ok {
			blockColor = customColor
			goto haveColor
		}

		blockColor = LookupColor(name)
		goto haveColor
	} else {
		blockColor = b.Color()
	}

haveColor:
	if blockColor.R == 0xff && blockColor.G == 0x0 && blockColor.B == 0xff {
		if updater.Version == "" {
			name, props := b.EncodeBlock()
			logrus.Infof("no color %s %v", name, props)
			b.Color()
		}
	}

	if blockColor.A != 0xff {
		blockColor = BlendColors(cr.blockColorAt(c, x, y-1, z), blockColor)
	}
	return blockColor
}

func (cr *ChunkRenderer) chunkGetColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) color.RGBA {
	br := c.BlockRegistry.(world.BlockRegistry)
	haveUp := false
	cube.Pos{int(x), int(y), int(z)}.
		Side(cube.FaceUp).
		Neighbours(func(neighbour cube.Pos) {
			if neighbour.X() < 0 || neighbour.X() >= 16 || neighbour.Z() < 0 || neighbour.Z() >= 16 || neighbour.Y() > c.Range().Max() || haveUp {
				return
			}
			blockRid := c.Block(uint8(neighbour[0]), int16(neighbour[1]), uint8(neighbour[2]), 0)
			if blockRid > 0 {
				b, found := br.BlockByRuntimeID(blockRid)
				if found {
					if isBlockLightblocking(b) {
						haveUp = true
					}
				}
			}
		}, cube.Range{int(y + 1), int(y + 1)})

	blockColor := cr.blockColorAt(c, x, y, z)
	if haveUp && (x+z)%2 == 0 {
		if blockColor.R > 10 {
			blockColor.R -= 10
		}
		if blockColor.G > 10 {
			blockColor.G -= 10
		}
		if blockColor.B > 10 {
			blockColor.B -= 10
		}
	}
	return blockColor
}

type ChunkRenderer struct {
	customBlockColors map[string]color.RGBA
}

func (cr *ChunkRenderer) ResolveColors(entries []protocol.BlockEntry, packs []resource.Pack) {
	colors := ResolveColors(entries, packs)
	cr.customBlockColors = colors
}

func (cr *ChunkRenderer) Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	hm := c.HeightMapWithWater()

	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			img.SetRGBA(
				int(x), int(z),
				cr.chunkGetColorAt(c, x, hm.At(x, z), z),
			)
		}
	}
	return img
}
