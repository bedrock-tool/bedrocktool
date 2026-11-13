package worldstate

import (
	"context"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func solidMapPacket(id int64, c color.RGBA) *packet.ClientBoundMapItemData {
	px := make([]color.RGBA, mapSize*mapSize)
	for i := range px {
		px[i] = c
	}
	// one odd pixel, so the map isn't blank
	px[0] = color.RGBA{1, 2, 3, 255}
	return &packet.ClientBoundMapItemData{
		MapID:   id,
		Width:   protocol.Option(int32(mapSize)),
		Height:  protocol.Option(int32(mapSize)),
		XOffset: protocol.Option(int32(0)),
		YOffset: protocol.Option(int32(0)),
		Pixels:  protocol.Option(px),
	}
}

func newTestWorld(t *testing.T) *World {
	w, err := New(context.Background(), nil, func(world.ChunkPos, *chunk.Chunk) {})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func frameNBT(p cube.Pos, mapID int64) map[string]any {
	return map[string]any{
		"id": "ItemFrame", "x": int32(p.X()), "y": int32(p.Y()), "z": int32(p.Z()),
		"Item": map[string]any{"Name": "minecraft:filled_map", "tag": map[string]any{"map_uuid": mapID}},
	}
}

func TestMapEncodes(t *testing.T) {
	m := ApplyMapPacket(nil, solidMapPacket(7, color.RGBA{200, 10, 10, 255}))
	b, err := nbt.MarshalEncoding(m, nbt.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := nbt.UnmarshalEncoding(b, &back, nbt.LittleEndian); err != nil {
		t.Fatal(err)
	}
	arr, ok := back["colors"].([mapSize * mapSize * 4]byte)
	if !ok {
		t.Fatalf("colors is %T, want a byte array", back["colors"])
	}
	colors := arr[:]
	if len(colors) != mapSize*mapSize*4 || colors[4] != 200 || colors[7] != 255 {
		t.Fatalf("colors wrong: len %d, %v", len(colors), colors[:8])
	}
	if back["mapLocked"] != uint8(1) || back["mapId"] != int64(7) {
		t.Fatalf("mapLocked %v mapId %v", back["mapLocked"], back["mapId"])
	}
}

func TestApplyMapPacketCopies(t *testing.T) {
	a := ApplyMapPacket(nil, solidMapPacket(1, color.RGBA{10, 10, 10, 255}))
	b := ApplyMapPacket(a, solidMapPacket(1, color.RGBA{20, 20, 20, 255}))
	if a.Colors[4] != 10 || b.Colors[4] != 20 {
		t.Fatalf("a %d b %d", a.Colors[4], b.Colors[4])
	}
}

func TestMapFramesAndExport(t *testing.T) {
	w := newTestWorld(t)
	var seen []int64
	w.OnMapSeen = func(id int64) { seen = append(seen, id) }

	// A 2 wide wall facing south (facing unknown here: no block registry),
	// plus a lone frame and a chest holding a map.
	ch := &Chunk{Chunk: chunk.New(world.DefaultBlockRegistry, cube.Range{-64, 319}), BlockEntities: map[cube.Pos]map[string]any{}}
	for i, p := range []cube.Pos{{1, 70, 5}, {2, 70, 5}, {8, 64, 8}} {
		ch.BlockEntities[p] = frameNBT(p, int64(100+i))
	}
	// Hive leaves the namespace off
	ch.BlockEntities[cube.Pos{8, 64, 8}]["Item"].(map[string]any)["Name"] = "filled_map"
	ch.BlockEntities[cube.Pos{3, 64, 3}] = map[string]any{"id": "Chest", "Items": []any{
		map[string]any{"Name": "minecraft:filled_map", "tag": map[string]any{"map_uuid": int64(200)}},
		map[string]any{"Name": "minecraft:stone"},
	}}
	w.stateLock.Lock()
	w.scanMapsLocked(world.ChunkPos{0, 0}, ch)
	w.scanMapsLocked(world.ChunkPos{0, 0}, ch) // seen again: not asked twice
	w.stateLock.Unlock()
	if len(seen) != 4 {
		t.Fatalf("seen %v, want 4 maps once each", seen)
	}

	w.StoreMap(solidMapPacket(100, color.RGBA{255, 0, 0, 255}))
	w.StoreMap(solidMapPacket(101, color.RGBA{0, 0, 255, 255}))
	// 102 arrived while capturing an earlier world
	w.stateLock.Lock()
	w.importMapsLocked(map[int64]*Map{102: ApplyMapPacket(nil, solidMapPacket(102, color.RGBA{0, 255, 0, 255}))})
	w.stateLock.Unlock()
	// 200 never arrived: no tile for it

	dir := t.TempDir()
	grids, tiles, err := w.ExportMapArt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if grids != 1 || tiles != 3 {
		t.Fatalf("grids %d tiles %d, want 1 and 3", grids, tiles)
	}
	f, err := os.Open(filepath.Join(dir, "grid_x1_y70_z5.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 128 {
		t.Fatalf("grid is %v", b)
	}
	if r, _, _, _ := img.At(64, 64).RGBA(); r>>8 != 255 {
		t.Fatal("left tile should be map 100 (red)")
	}
	if _, _, b, _ := img.At(192, 64).RGBA(); b>>8 != 255 {
		t.Fatal("right tile should be map 101 (blue)")
	}
}

func TestLayoutFacing(t *testing.T) {
	// Facing north you look south, so higher x is further left.
	c1, _ := layout(MapFrame{Pos: cube.Pos{1, 0, 0}, Facing: 2})
	c2, _ := layout(MapFrame{Pos: cube.Pos{2, 0, 0}, Facing: 2})
	if c2 >= c1 {
		t.Fatal("facing north: x+1 should be to the left")
	}
	_, r1 := layout(MapFrame{Pos: cube.Pos{0, 5, 0}, Facing: 4})
	_, r2 := layout(MapFrame{Pos: cube.Pos{0, 6, 0}, Facing: 4})
	if r2 >= r1 {
		t.Fatal("higher frames should be higher rows")
	}
}

func TestConvertHiveSheep(t *testing.T) {
	w := newTestWorld(t)
	meta := protocol.NewEntityMetadata()
	meta[protocol.EntityDataKeyName] = "§aPlay"
	meta[protocol.EntityDataKeyColorIndex] = byte(3)
	meta.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagSheared)
	w.StoreEntity(1, &entity.Entity{RuntimeID: 1, UniqueID: 1, EntityType: "minecraft:sheep", Metadata: meta})
	w.StoreEntity(2, &entity.Entity{RuntimeID: 2, UniqueID: 2, EntityType: "minecraft:cow", Metadata: protocol.NewEntityMetadata()})
	// sheep with no name, or only colour codes, are hitboxes: left out
	w.StoreEntity(3, &entity.Entity{RuntimeID: 3, UniqueID: 3, EntityType: "minecraft:sheep", Metadata: protocol.NewEntityMetadata()})
	blank := protocol.NewEntityMetadata()
	blank[protocol.EntityDataKeyName] = "§r "
	w.StoreEntity(4, &entity.Entity{RuntimeID: 4, UniqueID: 4, EntityType: "minecraft:sheep", Metadata: blank})
	if n, dropped := w.convertHiveSheep(); n != 1 || dropped != 2 {
		t.Fatalf("converted %d dropped %d", n, dropped)
	}
	if _, ok := w.memState.entities[3]; ok {
		t.Fatal("unnamed sheep kept")
	}
	if _, ok := w.memState.entities[4]; ok {
		t.Fatal("blank-named sheep kept")
	}
	e := w.memState.entities[1]
	if e.EntityType != HiveTextEntity || e.Metadata[protocol.EntityDataKeyName] != "§aPlay" {
		t.Fatalf("got %s %v", e.EntityType, e.Metadata[protocol.EntityDataKeyName])
	}
	if _, ok := e.Metadata[protocol.EntityDataKeyColorIndex]; ok || e.Metadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagSheared) {
		t.Fatal("sheep data left on the name tag")
	}
	if w.memState.entities[2].EntityType != "minecraft:cow" {
		t.Fatal("cow changed")
	}
}

func TestPreparePlayers(t *testing.T) {
	w := newTestWorld(t)
	npc, real := uuid.New(), uuid.New()
	w.AddPlayer(&packet.AddPlayer{UUID: npc, Username: "NPC", EntityRuntimeID: 10, EntityMetadata: protocol.NewEntityMetadata()})
	w.AddPlayer(&packet.AddPlayer{UUID: real, Username: "Someone", EntityRuntimeID: 11, EntityMetadata: protocol.NewEntityMetadata()})
	kept := w.preparePlayers(func(id uuid.UUID) bool { return id == npc })
	if len(kept) != 1 || kept[0].UUID != npc {
		t.Fatalf("kept %v", kept)
	}
	if _, ok := w.memState.entities[11]; ok {
		t.Fatal("real player still in the world")
	}
	e := w.memState.entities[10]
	if e.EntityType != "bedrocktool_player:"+npc.String() || e.Metadata[protocol.EntityDataKeyName] != "NPC" {
		t.Fatalf("npc saved as %s %v", e.EntityType, e.Metadata[protocol.EntityDataKeyName])
	}
	// nil keeps no one
	w2 := newTestWorld(t)
	w2.AddPlayer(&packet.AddPlayer{UUID: npc, EntityRuntimeID: 1, EntityMetadata: protocol.NewEntityMetadata()})
	if len(w2.preparePlayers(nil)) != 0 || len(w2.memState.entities) != 0 {
		t.Fatal("nil filter kept a player")
	}
}
