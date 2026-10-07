package worldstate

import (
	"context"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
)

func TestSetBlockNBTMergeLoadedColumn(t *testing.T) {
	br := world.NewBlockRegistry()
	br.Finalize()
	dir := t.TempDir()
	db, err := (mcdb.Config{Blocks: br}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pos := cube.Pos{17, 64, -1}
	cp, _ := cubePosInChunk(pos)
	ch := chunk.New(br, world.Overworld.Range())
	rid, ok := br.StateToRuntimeID("minecraft:stone", map[string]any{})
	if !ok {
		t.Fatal("missing stone")
	}
	ch.SetBlock(1, 64, 15, 0, rid)
	initial := map[string]any{"id": "Sign", "Text": "old", "x": int32(17), "y": int32(64), "z": int32(-1)}
	if err := db.StoreColumn(cp, world.Overworld, &chunk.Column{Chunk: ch, BlockEntities: []chunk.BlockEntity{{Pos: pos, Data: initial}}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = (mcdb.Config{Blocks: br}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, err := New(context.Background(), nil, func(world.ChunkPos, *chunk.Chunk) {})
	if err != nil {
		t.Fatal(err)
	}
	defer w.cancelCtx(nil)
	w.SetDimension(world.Overworld)
	w.BlockRegistry, w.provider = br, db
	if err := w.SetBlockNBT(pos, map[string]any{"Text": "updated"}, true); err != nil {
		t.Fatal(err)
	}
	w.stateLock.Lock()
	err = w.storeMemToProvider()
	w.stateLock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := db.LoadColumn(cp, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	initial["Text"] = "updated"
	if len(loaded.BlockEntities) != 1 || !reflect.DeepEqual(loaded.BlockEntities[0].Data, initial) {
		t.Fatalf("persisted NBT = %#v, want %#v", loaded.BlockEntities, initial)
	}
}
