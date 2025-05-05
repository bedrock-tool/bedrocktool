package utils

import (
	"image"
	"image/draw"
	"image/png"
	"os"

	"github.com/OneOfOne/xxhash"
	"github.com/df-mc/dragonfly/server/world"
)

type TextureMap struct {
	BlockSize int
	Lookup    *image.RGBA
}

func NewTextureMap() *TextureMap {
	return &TextureMap{
		BlockSize: 16,
	}
}

type BlockRID = uint32
type TexMapIdx = uint32
type TexMapHash = uint64
type TexMapEntry struct {
	X           uint16
	Y           uint16
	Transparent bool
}

func hashImage(img image.Image) uint64 {
	h := xxhash.New64()
	switch img := img.(type) {
	case *image.RGBA:
		h.Write(img.Pix)
	case *image.Paletted:
		h.Write(img.Pix)
	}
	return h.Sum64()
}

func (t *TextureMap) SetTextures(blocks []world.Block, resolvedTextures map[string]image.Image) map[BlockRID]TexMapEntry {
	// resolvedTextures = map from block name -> block top texture

	var hashes = map[TexMapHash]image.Image{}
	var hashToRids = map[TexMapHash][]BlockRID{}
	var ridToIdx = map[BlockRID]TexMapEntry{}

	for rid, block := range blocks {
		name, _ := block.EncodeBlock()
		tex, ok := resolvedTextures[name]
		if ok {
			h := hashImage(tex)
			hashToRids[h] = append(hashToRids[h], uint32(rid))
			hashes[h] = tex
		}
	}

	t.Lookup = image.NewRGBA(image.Rect(0, 0, 1024, 512))
	i := 0
	for k, he := range hashToRids {
		tex := hashes[k]

		x := (i * t.BlockSize) % t.Lookup.Rect.Dx()
		y := (i * t.BlockSize) / t.Lookup.Rect.Dx() * t.BlockSize
		draw.Draw(t.Lookup, image.Rect(x, y, x+t.BlockSize, y+t.BlockSize), tex, image.Point{}, draw.Over)

		for _, v := range he {
			ridToIdx[v] = TexMapEntry{
				X:           uint16(x),
				Y:           uint16(y),
				Transparent: false,
			}
		}

		i++
	}

	if IsDebug() {
		f, err := os.Create("tex.png")
		if err != nil {
			panic(err)
		}
		png.Encode(f, t.Lookup)
		f.Close()
	}

	return ridToIdx
}
