package main

import (
	"image"
	"image/color"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	hm := c.HeightMap()
	hml := c.LiquidHeightMap()

	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			height := hm.At(x, z)
			height_liquid := hml.At(x, z)

			col := color.RGBA{0, 0, 0, 255}

			block_rid := c.Block(x, height, z, 0)
			b, found := world.BlockByRuntimeID(block_rid)
			if found {
				col = b.Color()
			}

			/*
				if col.R == 255 && col.B == 255 {
					name, nbt := b.EncodeBlock()
					fmt.Printf("unknown color %s %s %s\n", reflect.TypeOf(b), name, nbt)
					b.Color()
				}
			*/

			if height_liquid > height {
				bw := &block.Water{}
				wcol := bw.Color()
				col.R = col.R/2 + wcol.R/2
				col.G = col.G/2 + wcol.G/2
				col.B = col.B/2 + wcol.B/2
			}

			img.SetRGBA(int(x), int(z), col)
		}
	}
	return img
}
