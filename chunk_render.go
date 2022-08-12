package main

import (
	"image"
	"image/color"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	hm := c.HeightMap()

	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			height := hm.At(x, z)
			col := color.RGBA{uint8(height), 0, uint8(height), 255}

			block_rid := c.Block(x, height, z, 0)
			b, found := world.BlockByRuntimeID(block_rid)
			if found {
				col = b.Color()
			}

			img.SetRGBA(int(x), int(z), col)
		}
	}
	return img
}
