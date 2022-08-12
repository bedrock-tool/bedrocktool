package main

import (
	"image"
	"image/color"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

func Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			height := c.HighestBlock(x, z)
			block_rid := c.Block(x, height, z, 0)
			img.SetRGBA(int(x), int(z), color.RGBA{
				R: uint8(height),
				G: uint8(block_rid & 0xFF),
				B: uint8((block_rid >> 8) & 0xFF),
				A: 255,
			})
		}
	}
	return img
}
