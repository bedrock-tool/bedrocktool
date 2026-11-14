package worldstate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// DimensionScriptModules are the Script API modules of the dimension pack:
// @minecraft/server 2.8.0 (Minecraft 1.26.30) is the first stable version
// with DimensionRegistry, as Microsoft's custom dimensions sample uses it.
var DimensionScriptModules = []behaviourpack.ScriptModule{
	{Name: "@minecraft/server", Version: "2.8.0"},
}

// DimensionMinimumGameVersion is the Minecraft version the dimension pack needs.
var DimensionMinimumGameVersion = resource.Version{1, 26, 30}

// DimensionCommand is the command that teleports between the dimensions.
const DimensionCommand = "bedrocktool:dim"

// playerEyeHeight is how far above its feet a player's position is in packets.
const playerEyeHeight = 1.62

type dimensionEntry struct {
	Name  string             `json:"name"`
	ID    string             `json:"id"`
	Title string             `json:"title"`
	Spawn map[string]float32 `json:"spawn"`
}

func spawnOf(pos mgl32.Vec3) map[string]float32 {
	r := func(f float32) float32 { return float32(math.Round(float64(f)*100) / 100) }
	return map[string]float32{"x": r(pos.X()), "y": r(pos.Y() - playerEyeHeight), "z": r(pos.Z())}
}

// DimensionPack is a behavior pack for a world with merged dimensions: it
// registers them (the game ignores a dimension's chunks otherwise) and
// adds /bedrocktool:dim [name] (and /scriptevent bedrocktool:dim [name]) to
// list them and teleport between them.
//
// It is a pack of its own: the menu recorder's script is written for older
// script modules, and a pack has one set of them.
func DimensionPack(seed, mainTitle string, mainSpawn mgl32.Vec3, dims []MergedDimension) *behaviourpack.Pack {
	bp := behaviourpack.New("bedrocktool dimensions " + seed)
	bp.Manifest.Header.Name = "bedrocktool dimensions"
	bp.Manifest.Header.Description = locale.Loc("dims_pack_description", nil)
	bp.Manifest.Header.MinimumGameVersion = DimensionMinimumGameVersion
	bp.SetScriptModules("Registers the dimensions and the /"+DimensionCommand+" command", DimensionScriptModules...)
	bp.SetScript(DimensionScript(mainTitle, mainSpawn, dims))
	for _, d := range dims {
		b, _ := json.MarshalIndent(DimensionDefinition(d), "", "\t")
		bp.AddFile("dimensions/"+d.ShortName()+".json", b)
	}
	return bp
}

// DimensionDefinition is the dimension's JSON (format 1.26.60, Preview):
// newer versions of the game can define dimensions this way instead of by
// script; the script then finds it registered already.
func DimensionDefinition(d MergedDimension) map[string]any {
	components := map[string]any{
		"minecraft:generation": map[string]any{"generator_type": "void"},
	}
	minY, height := d.Range.Min(), d.Range.Height()+1
	if minY%16 == 0 && height%16 == 0 && minY >= -512 && height >= 16 && minY+height <= 512 {
		components["minecraft:dimension_height"] = map[string]any{"min_y": minY, "height_range": height}
	}
	return map[string]any{
		"format_version": "1.26.60",
		"minecraft:dimension": map[string]any{
			"description": map[string]any{"identifier": d.Identifier},
			"components":  components,
		},
	}
}

// DimensionScript is the dimension pack's scripts/main.js.
func DimensionScript(mainTitle string, mainSpawn mgl32.Vec3, dims []MergedDimension) string {
	entries := []dimensionEntry{{
		Name:  "overworld",
		ID:    "minecraft:overworld",
		Title: mainTitle,
		Spawn: spawnOf(mainSpawn),
	}}
	for _, d := range dims {
		entries = append(entries, dimensionEntry{
			Name:  d.ShortName(),
			ID:    d.Identifier,
			Title: d.Title,
			Spawn: spawnOf(d.Spawn),
		})
	}
	const name = "{name}"
	text := map[string]string{
		"description": locale.Loc("dims_cmd_desc", nil),
		"list":        locale.Loc("dims_list", locale.Strmap{"Command": "/" + DimensionCommand}),
		"teleported":  locale.Loc("dims_teleported", locale.Strmap{"Name": name}),
		"unknown":     locale.Loc("dims_unknown", locale.Strmap{"Name": name}),
		"notLoaded":   locale.Loc("dims_not_loaded", locale.Strmap{"Name": name}),
		"notPlayer":   locale.Loc("dims_not_player", nil),
		"failed":      locale.Loc("dims_teleport_failed", locale.Strmap{"Name": name}),
	}
	js := func(v any) string {
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	}
	var sb strings.Builder
	sb.WriteString("// Written by bedrocktool: the worlds of one session, merged into this world as\n")
	sb.WriteString("// custom dimensions. /" + DimensionCommand + " lists them, /" + DimensionCommand + " <name> teleports;\n")
	sb.WriteString("// /scriptevent " + DimensionCommand + " <name> does the same.\n\n")
	sb.WriteString(dimensionScriptHeader)
	fmt.Fprintf(&sb, "const COMMAND = %q;\n\n", DimensionCommand)
	fmt.Fprintf(&sb, "const DIMENSIONS = %s;\n\n", js(entries))
	fmt.Fprintf(&sb, "const TEXT = %s;\n\n", js(text))
	sb.WriteString(dimensionScriptBody)
	return sb.String()
}

const dimensionScriptHeader = `import { world, system, CommandPermissionLevel, CustomCommandParamType, CustomCommandStatus } from "@minecraft/server";

`

const dimensionScriptBody = `function fmt(t, name) {
  return t.split("{name}").join(name);
}

function find(name) {
  const n = String(name ?? "").trim().toLowerCase();
  return DIMENSIONS.find((d) => d.name === n || d.id === n);
}

function list() {
  return TEXT.list + "\n" + DIMENSIONS.map((d) => "§e" + d.name + "§r - " + d.title).join("\n");
}

function run(player, name) {
  if (name === undefined || String(name).trim() === "") {
    return { status: CustomCommandStatus.Success, message: list() };
  }
  const d = find(name);
  if (!d) {
    return { status: CustomCommandStatus.Failure, message: fmt(TEXT.unknown, String(name)) + "\n" + list() };
  }
  if (!player || player.typeId !== "minecraft:player") {
    return { status: CustomCommandStatus.Failure, message: TEXT.notPlayer };
  }
  let dimension;
  try {
    dimension = world.getDimension(d.id);
  } catch (e) {
    return { status: CustomCommandStatus.Failure, message: fmt(TEXT.notLoaded, d.id) };
  }
  // commands can't change the world themselves: teleport on the next tick
  system.run(() => {
    try {
      player.teleport(d.spawn, { dimension });
      player.sendMessage(fmt(TEXT.teleported, d.title));
    } catch (e) {
      player.sendMessage(fmt(TEXT.failed, d.title) + " " + e);
    }
  });
  return { status: CustomCommandStatus.Success };
}

system.beforeEvents.startup.subscribe((ev) => {
  for (const d of DIMENSIONS) {
    if (d.id.startsWith("minecraft:")) continue;
    try {
      ev.dimensionRegistry.registerCustomDimension(d.id);
    } catch (e) {
      // already defined by dimensions/*.json (newer versions), or not supported
      console.warn("bedrocktool: " + d.id + ": " + e);
    }
  }
  ev.customCommandRegistry.registerEnum("bedrocktool:dimension", DIMENSIONS.map((d) => d.name));
  ev.customCommandRegistry.registerCommand(
    {
      name: COMMAND,
      description: TEXT.description,
      permissionLevel: CommandPermissionLevel.Any,
      cheatsRequired: false,
      optionalParameters: [{ type: CustomCommandParamType.Enum, name: "bedrocktool:dimension" }],
    },
    (origin, name) => run(origin.sourceEntity, name),
  );
});

system.afterEvents.scriptEventReceive.subscribe((ev) => {
  if (ev.id !== COMMAND) return;
  const player = ev.sourceEntity ?? world.getAllPlayers()[0];
  const result = run(player, ev.message);
  if (result.message && player) player.sendMessage(result.message);
});
`

// DimensionPackFolder is where the dimension pack goes in a world.
const DimensionPackFolder = "behavior_packs/bedrocktool_dimensions"

// WriteDimensionPack writes the dimension pack for dims into the world in
// folder (replacing an older one) and turns it on in world_behavior_packs.json.
func WriteDimensionPack(folder, seed, mainTitle string, mainSpawn mgl32.Vec3, dims []MergedDimension) error {
	bp := DimensionPack(seed, mainTitle, mainSpawn, dims)
	dir := filepath.Join(folder, filepath.FromSlash(DimensionPackFolder))
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := bp.Save(utils.OSWriter{Base: dir}); err != nil {
		return err
	}
	h := bp.Manifest.Header
	return AddWorldPacks(folder, "world_behavior_packs.json", []resourcePackDependency{{
		UUID:    h.UUID.String(),
		Version: h.Version,
	}})
}
