package worlds

import (
	"fmt"
	"image"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func Test_chunkPosToTilePos(t *testing.T) {
	type test struct {
		pos            protocol.ChunkPos
		expectedTile   image.Point
		expectedOffset image.Point
	}

	const chunksPerTile = tileSize / 16

	var tests = []test{
		{
			pos:            protocol.ChunkPos{0, 0},
			expectedTile:   image.Pt(0, 0),
			expectedOffset: image.Pt(0, 0),
		},
		{
			pos:            protocol.ChunkPos{1, 0},
			expectedTile:   image.Pt(0, 0),
			expectedOffset: image.Pt(16, 0),
		},
		{
			pos:            protocol.ChunkPos{2, 0},
			expectedTile:   image.Pt(0, 0),
			expectedOffset: image.Pt(32, 0),
		},
		{
			pos:            protocol.ChunkPos{1, 1},
			expectedTile:   image.Pt(0, 0),
			expectedOffset: image.Pt(16, 16),
		},
		{
			pos:            protocol.ChunkPos{chunksPerTile, 1},
			expectedTile:   image.Pt(1, 0),
			expectedOffset: image.Pt(0, 16),
		},
		{
			pos:            protocol.ChunkPos{-1, 1},
			expectedTile:   image.Pt(-1, 0),
			expectedOffset: image.Pt(tileSize-16, 16),
		},
		{
			pos:            protocol.ChunkPos{-1, -1},
			expectedTile:   image.Pt(-1, -1),
			expectedOffset: image.Pt(tileSize-16, tileSize-16),
		},
		{
			pos:            protocol.ChunkPos{-2, -1},
			expectedTile:   image.Pt(-1, -1),
			expectedOffset: image.Pt(tileSize-32, tileSize-16),
		},
	}

	for _, t2 := range tests {
		tile, offset := chunkPosToTilePos(t2.pos)
		if t2.expectedOffset != offset {
			t.Error(fmt.Errorf("%+v wrong offset %v", t2, offset))
		}
		if t2.expectedTile != tile {
			t.Error(fmt.Errorf("%+v wrong tile %v", t2, tile))
		}
	}
}
