package main

import (
	"image"
	"image/color"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func chunkGetColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) color.RGBA {
	p := cube.Pos{int(x), int(y), int(z)}
	have_up := false
	p.Side(cube.FaceUp).Neighbours(func(neighbour cube.Pos) {
		if !have_up {
			block_rid := c.Block(uint8(neighbour[0]), int16(neighbour[1]), uint8(neighbour[2]), 0)
			b, found := world.BlockByRuntimeID(block_rid)
			if found {
				if _, ok := b.(block.Air); !ok {
					if _, ok := b.(block.Water); !ok {
						have_up = true
					}
				}
			}
		}
	}, cube.Range{int(y + 1), int(y + 1)})

	col := color.RGBA{0, 0, 0, 255}
	block_rid := c.Block(x, y, z, 0)
	if block_rid == 0 && y == 0 { // void
		col = color.RGBA{0, 0, 0, 255}
	} else {
		b, found := world.BlockByRuntimeID(block_rid)
		if found {
			col = b.Color()
		}
		/*
			if col.R == 255 && col.B == 255 {
				name, nbt := b.EncodeBlock()
				fmt.Printf("unknown color %d  %s %s %s\n", block_rid, reflect.TypeOf(b), name, nbt)
				b.Color()
			}
		*/
	}

	if have_up {
		col.R -= 10
		col.G -= 10
		col.B -= 10
	}
	return col
}

func Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	hm := c.HeightMap()
	hml := c.LiquidHeightMap()

	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			height := hm.At(x, z)
			height_liquid := hml.At(x, z)

			col := chunkGetColorAt(c, x, height, z)

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
