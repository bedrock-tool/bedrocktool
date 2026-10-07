package worldstate

import (
	"context"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func TestSetBlockNBTMerge(t *testing.T) {
	pos := cube.Pos{17, 64, -1}
	for _, tc := range []struct {
		name     string
		entities map[cube.Pos]map[string]any
		want     map[string]any
	}{
		{"absent NBT", map[cube.Pos]map[string]any{}, map[string]any{"Text": "updated"}},
		{"nil NBT", map[cube.Pos]map[string]any{pos: nil}, map[string]any{"Text": "updated"}},
		{"nil parent", nil, map[string]any{"Text": "updated"}},
		{"partial update", map[cube.Pos]map[string]any{pos: {"id": "Sign", "Text": "old", "x": int32(17)}}, map[string]any{"id": "Sign", "Text": "updated", "x": int32(17)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, err := New(context.Background(), nil, func(world.ChunkPos, *chunk.Chunk) {})
			if err != nil {
				t.Fatal(err)
			}
			defer w.cancelCtx(nil)
			w.SetDimension(world.Overworld)
			br := world.NewBlockRegistry()
			br.Finalize()
			w.BlockRegistry = br
			cp, _ := cubePosInChunk(pos)
			ch := &Chunk{Chunk: chunk.New(br, w.Range()), BlockEntities: tc.entities}
			if err := w.StoreChunk(cp, ch); err != nil {
				t.Fatal(err)
			}
			update := map[string]any{"Text": "updated"}
			if err := w.SetBlockNBT(pos, update, true); err != nil {
				t.Fatal(err)
			}
			loaded, ok, err := w.LoadChunk(cp)
			if err != nil || !ok {
				t.Fatalf("load: ok=%v err=%v", ok, err)
			}
			if got := loaded.BlockEntities[pos]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NBT = %#v, want %#v", got, tc.want)
			}
			update["Text"] = "caller mutation"
			if loaded.BlockEntities[pos]["Text"] != "updated" {
				t.Fatal("merged NBT aliases caller map")
			}
			if err := w.SetBlockNBT(pos, map[string]any{"extra": int32(1)}, true); err != nil {
				t.Fatal(err)
			}
			if _, ok := update["extra"]; ok {
				t.Fatal("subsequent merge mutated caller map")
			}
			if loaded.BlockEntities[pos]["Text"] != "updated" {
				t.Fatal("subsequent merge lost existing fields")
			}
		})
	}
}
