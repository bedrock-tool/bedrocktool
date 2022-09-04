package world

import (
	"image"
	"image/color"

	"bedrocktool/cmd/bedrocktool/utils"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func blockColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) color.RGBA {
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
	return col
}

func chunkGetColorAt(c *chunk.Chunk, x uint8, y int16, z uint8) color.RGBA {
	p := cube.Pos{int(x), int(y), int(z)}
	have_up := false
	p.Side(cube.FaceUp).Neighbours(func(neighbour cube.Pos) {
		if neighbour.X() < 0 || neighbour.X() >= 16 || neighbour.Z() < 0 || neighbour.Z() >= 16 {
			return
		}
		if !have_up {
			block_rid := c.Block(uint8(neighbour[0]), int16(neighbour[1]), uint8(neighbour[2]), 0)
			if block_rid > 0 {
				b, found := world.BlockByRuntimeID(block_rid)
				if found {
					if _, ok := b.(block.Air); !ok {
						have_up = true
					}
				}
			}
		}
	}, cube.Range{int(y + 1), int(y + 1)})

	col := blockColorAt(c, x, y, z)

	if have_up {
		if col.R > 10 {
			col.R -= 10
		}
		if col.G > 10 {
			col.G -= 10
		}
		if col.B > 10 {
			col.B -= 10
		}
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
				bw := (&block.Water{}).Color()
				bw.A = uint8(utils.Clamp(int(127+(height_liquid-height)*5), 255))
				col = utils.BlendColors(col, bw)
			}

			img.SetRGBA(int(x), int(z), col)
		}
	}
	return img
}