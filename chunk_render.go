package main

import (
	"hash/crc32"
	"image"
	"image/color"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

func i32tob(val uint32) []byte {
	r := make([]byte, 4)
	for i := uint32(0); i < 4; i++ {
		r[i] = byte((val >> (8 * i)) & 0xff)
	}
	return r
}

func calcColor(clr int) color.RGBA {
	return color.RGBA{
		R: uint8((clr >> 24) & 0xFF),
		G: uint8((clr >> 16) & 0xFF),
		B: uint8((clr >> 8) & 0xFF),
		A: 255,
	}
}

func Chunk2Img(c *chunk.Chunk) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			block_rid := c.Block(x, c.HighestBlock(x, z), z, 0)
			col := crc32.ChecksumIEEE(i32tob(uint32(block_rid)))
			img.SetRGBA(int(x), int(z), calcColor(int(col)))
		}
	}
	return img
}
