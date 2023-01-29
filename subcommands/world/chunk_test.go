package world_test

import (
	"image/png"
	"os"
	"testing"

	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func Test(t *testing.T) {
	data, _ := os.ReadFile("chunk.bin")
	ch, _, _ := chunk.NetworkDecode(33, data, 6, cube.Range{0, 255}, true, false)
	i := world.Chunk2Img(ch)
	f, _ := os.Create("chunk.png")
	png.Encode(f, i)
	f.Close()
}

func Benchmark_chunk_decode(b *testing.B) {
	data, _ := os.ReadFile("chunk.bin")
	for i := 0; i < b.N; i++ {
		_, _, err := chunk.NetworkDecode(33, data, 6, cube.Range{0, 255}, true, false)
		if err != nil {
			b.Error(err)
		}
	}
}

func Benchmark_render_chunk(b *testing.B) {
	data, _ := os.ReadFile("chunk.bin")
	ch, _, _ := chunk.NetworkDecode(33, data, 6, cube.Range{0, 255}, true, false)

	for i := 0; i < b.N; i++ {
		world.Chunk2Img(ch)
	}
}
