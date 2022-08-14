package main

import (
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func Benchmark_chunk_decode(b *testing.B) {
	data, _ := os.ReadFile("chunk.bin")
	for i := 0; i < b.N; i++ {
		_, err := chunk.NetworkDecode(33, data, 6, cube.Range{-64, 319})
		if err != nil {
			b.Error(err)
		}
	}
}

func Benchmark_render_chunk(b *testing.B) {
	data, _ := os.ReadFile("chunk.bin")
	ch, _ := chunk.NetworkDecode(33, data, 6, cube.Range{-64, 319})

	for i := 0; i < b.N; i++ {
		Chunk2Img(ch)
	}
}
