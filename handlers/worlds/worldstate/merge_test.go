package worldstate

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/dop251/goja"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

func testRegistry() *merge.BlockRegistry {
	return &merge.BlockRegistry{BlockRegistry: world.DefaultBlockRegistry, Rids: make(map[uint32]merge.Block)}
}

// makeWorld saves a world with one block at (1, y, 1) in chunk 0,0 of dim,
// the entities given and a map.
func makeWorld(t *testing.T, dir string, dim world.Dimension, block string, y int16, entities []chunk.Entity, mapID int64) {
	t.Helper()
	br := testRegistry()
	db, err := mcdb.Config{Log: slog.Default(), Blocks: br}.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	rid, ok := br.StateToRuntimeID(block, nil)
	if !ok {
		t.Fatalf("no block %s", block)
	}
	ch := chunk.New(br, dim.Range())
	ch.SetBlock(1, y, 1, 0, rid)
	for by := dim.Range().Min(); by <= dim.Range().Max(); by++ {
		ch.SetBiome(1, int16(by), 1, 8) // nether wastes
	}
	col := &chunk.Column{
		Chunk:         ch,
		BlockEntities: []chunk.BlockEntity{{Pos: cube.Pos{1, int(y) + 1, 1}, Data: map[string]any{"id": "Sign", "x": int32(1), "y": int32(y + 1), "z": int32(1)}}},
		Entities:      entities,
	}
	if err := db.StoreColumn(world.ChunkPos{0, 0}, dim, col); err != nil {
		t.Fatal(err)
	}
	m := NewMap(mapID)
	data, err := nbt.MarshalEncoding(m, nbt.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.LDB().Put([]byte("map_"+itoa(mapID)), data, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func itoa(i int64) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func testEntity(id int64, typ string, links ...int64) chunk.Entity {
	data := map[string]any{"identifier": typ, "Pos": []any{float32(1), float32(70), float32(1)}}
	if len(links) > 0 {
		var l []any
		for _, id := range links {
			l = append(l, map[string]any{"entityID": id, "LinkID": int32(0)})
		}
		data["LinksTag"] = l
	}
	return chunk.Entity{ID: id, Data: data}
}

// A world merged into another lands in its own dimension with its
// blocks, block entities, entities (a clashing id renamed, rides kept) and
// maps, and the main world's overworld stays as it was.
func TestMergeWorld(t *testing.T) {
	mainDir, srcDir := t.TempDir(), t.TempDir()
	makeWorld(t, mainDir, world.Overworld, "minecraft:stone", 64, []chunk.Entity{testEntity(5, "minecraft:pig")}, 7)
	makeWorld(t, srcDir, world.Overworld, "minecraft:gold_block", 10, []chunk.Entity{
		testEntity(5, "minecraft:boat", 6), // the same id as the main world's pig
		testEntity(6, "minecraft:zombie"),
	}, 8)

	dim := MergedDimension{ID: FirstCustomDimensionID, Identifier: "bedrocktool:world_1", Title: "world-1", Range: cube.Range{-64, 319}}
	stats, err := MergeWorld(mainDir, srcDir, 0, world.Overworld.Range(), dim)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Chunks != 1 || stats.Entities != 2 || stats.Maps != 1 || stats.RenamedIDs != 1 {
		t.Fatalf("stats: %+v", stats)
	}

	table, err := ReadDimensionTable(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 1 || table["bedrocktool:world_1"] != FirstCustomDimensionID {
		t.Fatalf("table: %v", table)
	}

	dims, err := RegisterWorldDimensions(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	custom, ok := FindDimension(dims, "world_1")
	if !ok || custom.ID != FirstCustomDimensionID {
		t.Fatalf("dims: %+v", dims)
	}
	// reading it again (render run twice in the GUI) is fine
	if _, err := RegisterWorldDimensions(mainDir); err != nil {
		t.Fatal(err)
	}

	br := testRegistry()
	db, err := mcdb.Config{Log: slog.Default(), Blocks: br}.Open(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	blockAt := func(col *chunk.Column, y int16) string {
		name, _, _ := br.RuntimeIDToState(col.Chunk.Block(1, y, 1, 0))
		return name
	}
	main, err := db.LoadColumn(world.ChunkPos{0, 0}, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	if blockAt(main, 64) != "minecraft:stone" || len(main.Entities) != 1 || main.Entities[0].ID != 5 {
		t.Fatalf("main world changed: %s, %+v", blockAt(main, 64), main.Entities)
	}

	col, err := db.LoadColumn(world.ChunkPos{0, 0}, custom.Dimension)
	if err != nil {
		t.Fatal(err)
	}
	if got := blockAt(col, 10); got != "minecraft:gold_block" {
		t.Fatalf("merged block: %s", got)
	}
	if len(col.BlockEntities) != 1 {
		t.Fatalf("block entities: %+v", col.BlockEntities)
	}
	if len(col.Entities) != 2 {
		t.Fatalf("entities: %+v", col.Entities)
	}
	var boat, zombie chunk.Entity
	for _, e := range col.Entities {
		switch e.Data["identifier"] {
		case "minecraft:boat":
			boat = e
		case "minecraft:zombie":
			zombie = e
		}
	}
	if boat.ID == 5 || boat.ID == 0 || zombie.ID != 6 {
		t.Fatalf("ids: boat %d zombie %d", boat.ID, zombie.ID)
	}
	if uid, _ := boat.Data["UniqueID"].(int64); uid != boat.ID {
		t.Fatalf("boat UniqueID %v, key %d", boat.Data["UniqueID"], boat.ID)
	}
	links, _ := boat.Data["LinksTag"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["entityID"] != int64(6) {
		t.Fatalf("boat links: %v", boat.Data["LinksTag"])
	}
	for _, id := range []string{"map_7", "map_8"} {
		if ok, _ := db.LDB().Has([]byte(id), nil); !ok {
			t.Fatalf("%s missing", id)
		}
	}
	if ok, _ := db.LDB().Has([]byte("bedrocktool:world_1"), nil); !ok {
		t.Fatal("dimension key missing")
	}
}

// A world saved in the Nether (0 to 127 high) becomes a dimension of the
// default height (-64 to 319): its blocks stay where they were and its
// biomes are moved to match.
func TestMergeWorldFromNether(t *testing.T) {
	mainDir, srcDir := t.TempDir(), t.TempDir()
	makeWorld(t, mainDir, world.Overworld, "minecraft:stone", 64, nil, 1)
	makeWorld(t, srcDir, world.Nether, "minecraft:netherrack", 40, nil, 2)
	if r := TargetRange(world.Nether.Range()); r != DefaultDimensionRange {
		t.Fatalf("target range %v", r)
	}
	dim := MergedDimension{ID: 1003, Identifier: "bedrocktool:nether_world", Range: DefaultDimensionRange}
	stats, err := MergeWorld(mainDir, srcDir, 1, world.Nether.Range(), dim)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Chunks != 1 {
		t.Fatalf("stats: %+v", stats)
	}

	dims, err := RegisterWorldDimensions(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := FindDimension(dims, "nether_world")
	if !ok || d.Dimension.Range() != DefaultDimensionRange {
		t.Fatalf("dims: %+v", dims)
	}
	br := testRegistry()
	db, err := mcdb.Config{Log: slog.Default(), Blocks: br}.Open(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	col, err := db.LoadColumn(world.ChunkPos{0, 0}, d.Dimension)
	if err != nil {
		t.Fatal(err)
	}
	if name, _, _ := br.RuntimeIDToState(col.Chunk.Block(1, 40, 1, 0)); name != "minecraft:netherrack" {
		t.Fatalf("block at 40: %s", name)
	}
	for _, y := range []int16{-60, 0, 40, 127, 300} {
		if b := col.Chunk.Biome(1, y, 1); b != 8 {
			t.Errorf("biome at %d: %d", y, b)
		}
	}
	// a world taller than the default keeps its height
	if r := TargetRange(cube.Range{-128, 383}); r != (cube.Range{-128, 383}) {
		t.Fatalf("tall range %v", r)
	}
}

func TestDimensionIdentifier(t *testing.T) {
	taken := map[string]bool{}
	for _, tc := range []struct{ in, want string }{
		{"world-2", "bedrocktool:world_2"},
		{"Bed Wars: Lobby!", "bedrocktool:bed_wars_lobby"},
		{"world-2", "bedrocktool:world_2_2"},
		{"Overworld", "bedrocktool:world_overworld"},
		{"§a★", "bedrocktool:world"},
	} {
		got := DimensionIdentifier(tc.in, taken)
		if got != tc.want {
			t.Errorf("%q: %s, want %s", tc.in, got, tc.want)
		}
		taken[got] = true
	}
}

// The dimension pack's manifest needs @minecraft/server 2.8.0, and its
// script registers the dimensions and the command, lists them and
// teleports (run with stand-ins for the game's modules).
func TestDimensionPack(t *testing.T) {
	dims := []MergedDimension{
		{ID: 1000, Identifier: "bedrocktool:world_1", Title: "world-1", Spawn: mgl32.Vec3{10, 71.62, 20}, Range: cube.Range{-64, 319}},
		{ID: 1001, Identifier: "bedrocktool:sky", Title: "Sky \"Wars\"", Spawn: mgl32.Vec3{0, 101.62, 0}, Range: cube.Range{0, 255}},
	}
	dir := t.TempDir()
	if err := WriteDimensionPack(dir, "test", "Hub", mgl32.Vec3{1, 65.62, 2}, dims); err != nil {
		t.Fatal(err)
	}
	packDir := filepath.Join(dir, filepath.FromSlash(DimensionPackFolder))

	var manifest struct {
		Header struct {
			UUID          string `json:"uuid"`
			MinEngine     []int  `json:"min_engine_version"`
			FormatVersion int    `json:"format_version"`
		} `json:"header"`
		Modules []struct {
			Type  string `json:"type"`
			Entry string `json:"entry"`
		} `json:"modules"`
		Dependencies []struct {
			ModuleName string `json:"module_name"`
			Version    any    `json:"version"`
		} `json:"dependencies"`
	}
	readJSON(t, filepath.Join(packDir, "manifest.json"), &manifest)
	if len(manifest.Header.MinEngine) != 3 || manifest.Header.MinEngine[1] != 26 || manifest.Header.MinEngine[2] != 30 {
		t.Fatalf("min engine: %v", manifest.Header.MinEngine)
	}
	var hasScript, hasServer bool
	for _, m := range manifest.Modules {
		hasScript = hasScript || (m.Type == "script" && m.Entry == "scripts/main.js")
	}
	for _, d := range manifest.Dependencies {
		if d.ModuleName == "@minecraft/server-ui" {
			t.Fatal("server-ui isn't used")
		}
		hasServer = hasServer || (d.ModuleName == "@minecraft/server" && d.Version == "2.8.0")
	}
	if !hasScript || !hasServer {
		t.Fatalf("manifest: %+v", manifest)
	}

	var packs []resourcePackDependency
	readJSON(t, filepath.Join(dir, "world_behavior_packs.json"), &packs)
	if len(packs) != 1 || !strings.EqualFold(packs[0].UUID, manifest.Header.UUID) {
		t.Fatalf("world_behavior_packs.json: %+v", packs)
	}
	// writing it again (another dimension merged) replaces it
	if err := WriteDimensionPack(dir, "test", "Hub", mgl32.Vec3{1, 65.62, 2}, dims); err != nil {
		t.Fatal(err)
	}
	readJSON(t, filepath.Join(dir, "world_behavior_packs.json"), &packs)
	if len(packs) != 1 {
		t.Fatalf("pack listed twice: %+v", packs)
	}

	var def map[string]any
	readJSON(t, filepath.Join(packDir, "dimensions", "sky.json"), &def)
	height := def["minecraft:dimension"].(map[string]any)["components"].(map[string]any)["minecraft:dimension_height"].(map[string]any)
	if height["min_y"] != float64(0) || height["height_range"] != float64(256) {
		t.Fatalf("sky height: %v", height)
	}
	heights := readDimensionHeights(dir)
	if heights["bedrocktool:sky"] != (cube.Range{0, 255}) {
		t.Fatalf("heights: %v", heights)
	}

	script, err := os.ReadFile(filepath.Join(packDir, "scripts", "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	runDimensionScript(t, string(script))
}

func readJSON(t *testing.T, file string, v any) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %s", file, err)
	}
}

// stand-ins for @minecraft/server, recording what the script does
const fakeServer = `
const log = [];
const CommandPermissionLevel = { Any: 0, GameDirectors: 1 };
const CustomCommandParamType = { Enum: "enum", String: "string" };
const CustomCommandStatus = { Success: 0, Failure: 1 };
const registered = new Set();
let startup, scriptEvent, command;
const world = {
  getDimension(id) {
    if (id !== "minecraft:overworld" && !registered.has(id)) throw new Error("no dimension " + id);
    return { id };
  },
  getAllPlayers() { return [player]; },
};
const system = {
  beforeEvents: { startup: { subscribe(f) { startup = f; } } },
  afterEvents: { scriptEventReceive: { subscribe(f) { scriptEvent = f; } } },
  run(f) { f(); },
};
const player = {
  typeId: "minecraft:player",
  teleport(pos, opts) { log.push(["tp", pos, opts.dimension.id]); },
  sendMessage(m) { log.push(["msg", m]); },
};
const console = { warn(m) { log.push(["warn", m]); } };
`

const fakeStartup = `
const enums = {};
startup({
  dimensionRegistry: { registerCustomDimension(id) {
    if (registered.has(id)) throw new Error("CustomDimensionAlreadyRegisteredError");
    registered.add(id);
  } },
  customCommandRegistry: {
    registerEnum(name, values) { enums[name] = values; },
    registerCommand(def, cb) { command = { def, cb }; },
  },
});
`

func runDimensionScript(t *testing.T, script string) {
	t.Helper()
	var body []string
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(line, "import ") {
			body = append(body, line)
		}
	}
	vm := goja.New()
	if _, err := vm.RunString(fakeServer + strings.Join(body, "\n") + fakeStartup); err != nil {
		t.Fatalf("script: %s", err)
	}
	eval := func(js string) goja.Value {
		t.Helper()
		v, err := vm.RunString(js)
		if err != nil {
			t.Fatalf("%s: %s", js, err)
		}
		return v
	}

	if got := eval(`[...registered].join(",")`).String(); got != "bedrocktool:world_1,bedrocktool:sky" {
		t.Fatalf("registered: %s", got)
	}
	if got := eval(`enums["bedrocktool:dimension"].join(",")`).String(); got != "overworld,world_1,sky" {
		t.Fatalf("enum: %s", got)
	}
	if got := eval(`command.def.name + " " + command.def.cheatsRequired + " " + command.def.optionalParameters[0].name`).String(); got != DimensionCommand+" false bedrocktool:dimension" {
		t.Fatalf("command: %s", got)
	}

	// no name lists them
	list := eval(`JSON.stringify(command.cb({ sourceEntity: player }))`).String()
	if !strings.Contains(list, "world_1") || !strings.Contains(list, `Sky \"Wars\"`) || !strings.Contains(list, `"status":0`) {
		t.Fatalf("list: %s", list)
	}
	// a name teleports to the spawn, at the feet
	eval(`log.length = 0; command.cb({ sourceEntity: player }, "world_1")`)
	if got := eval(`JSON.stringify(log[0])`).String(); got != `["tp",{"x":10,"y":70,"z":20},"bedrocktool:world_1"]` {
		t.Fatalf("teleport: %s", got)
	}
	eval(`log.length = 0; command.cb({ sourceEntity: player }, "overworld")`)
	if got := eval(`JSON.stringify(log[0])`).String(); got != `["tp",{"x":1,"y":64,"z":2},"minecraft:overworld"]` {
		t.Fatalf("teleport home: %s", got)
	}
	if got := eval(`command.cb({ sourceEntity: player }, "nope").status`).ToInteger(); got != 1 {
		t.Fatalf("unknown name status %d", got)
	}
	if got := eval(`command.cb({ sourceEntity: undefined }, "sky").status`).ToInteger(); got != 1 {
		t.Fatalf("no player status %d", got)
	}
	// /scriptevent does the same
	eval(`log.length = 0; scriptEvent({ id: "bedrocktool:dim", message: " sky ", sourceEntity: player })`)
	if got := eval(`log[0][2]`).String(); got != "bedrocktool:sky" {
		t.Fatalf("scriptevent: %s", eval(`JSON.stringify(log)`))
	}
	// already registered by dimensions/*.json: the script carries on
	vm2 := goja.New()
	if _, err := vm2.RunString(fakeServer + `registered.add("bedrocktool:sky");` + strings.Join(body, "\n") + fakeStartup + `if (!command) throw new Error("no command");`); err != nil {
		t.Fatalf("script with the dimension already registered: %s", err)
	}
}
