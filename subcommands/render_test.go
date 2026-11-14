package subcommands

import (
	"fmt"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

func framedMap(p cube.Pos, id int64) chunk.BlockEntity {
	return chunk.BlockEntity{Pos: p, Data: map[string]any{
		"id": "ItemFrame", "x": int32(p.X()), "y": int32(p.Y()), "z": int32(p.Z()),
		"Item": map[string]any{"Name": "minecraft:filled_map", "tag": map[string]any{"map_uuid": id}},
	}}
}

// A saved world with a wall of two framed maps and a map in a chest gives
// a tile per map and one stitched wall.
func TestExportWorldMapArt(t *testing.T) {
	dir := t.TempDir()
	br := &merge.BlockRegistry{BlockRegistry: world.DefaultBlockRegistry, Rids: make(map[uint32]merge.Block)}
	db, err := mcdb.Config{Log: slog.Default(), Blocks: br}.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	col := &chunk.Column{
		Chunk: chunk.New(br, world.Overworld.Range()),
		BlockEntities: []chunk.BlockEntity{
			framedMap(cube.Pos{1, 70, 1}, 11),
			framedMap(cube.Pos{2, 70, 1}, 12),
			{Pos: cube.Pos{5, 64, 5}, Data: map[string]any{"id": "Chest", "Items": []any{
				map[string]any{"Name": "minecraft:filled_map", "tag": map[string]any{"map_uuid": int64(13)}},
			}}},
		},
	}
	if err := db.StoreColumn(world.ChunkPos{0, 0}, world.Overworld, col); err != nil {
		t.Fatal(err)
	}
	for i, id := range []int64{11, 12, 13} {
		m := worldstate.NewMap(id)
		for p := 0; p < len(m.Colors); p += 4 {
			m.Colors[p], m.Colors[p+3] = byte(60*i+40), 255
		}
		m.Colors[0] = 1 // not one colour, so not blank
		data, err := nbt.MarshalEncoding(m, nbt.LittleEndian)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.LDB().Put([]byte(fmt.Sprintf("map_%d", id)), data, nil); err != nil {
			t.Fatal(err)
		}
	}

	out := t.TempDir()
	err = exportWorldMapArt(db, br, &RenderSettings{Out: filepath.Join(out, "world.png")})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	art := filepath.Join(out, "world_mapart")
	for _, id := range []int64{11, 12, 13} {
		if _, err := os.Stat(filepath.Join(art, "tiles", fmt.Sprintf("%d.png", id))); err != nil {
			t.Fatalf("map %d: %s", id, err)
		}
	}
	walls, _ := filepath.Glob(filepath.Join(art, "grid_*.png"))
	if len(walls) != 1 {
		t.Fatalf("want 1 wall, got %v", walls)
	}
	f, _ := os.Open(walls[0])
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 256 || cfg.Height != 128 {
		t.Fatalf("the wall should be two maps side by side (256x128), got %dx%d %v", cfg.Width, cfg.Height, err)
	}
}
