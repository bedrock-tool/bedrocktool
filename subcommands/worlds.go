package subcommands

import (
	"context"
	"os"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type WorldSettings struct {
	ProxySettings proxy.ProxySettings
	Generator     string   `opt:"Unsaved Chunks" flag:"generator" default:"void" choices:"void,flat,normal" desc:"What Minecraft fills the chunks you didn't load with: void (empty), flat (superflat land) or normal (terrain from the server's seed). Many servers send a fake seed or 0, so normal terrain often won't match the saved area"`
	Void          bool     `flag:"void" default:"true" hidden:"true" desc:"Old switch: -void=false is -generator flat"`
	Image         bool     `opt:"Map Image" flag:"image" desc:"locale.save_image"`
	Entities      bool     `opt:"Entities" flag:"save-entities" default:"true" desc:"Mobs, armor stands, item frames and other entities"`
	Players       bool     `opt:"Players" flag:"save-players" default:"false" desc:"Other players, as statues wearing their skins"`
	MapArt        bool     `opt:"Map Art" flag:"map-art" default:"false" desc:"Also save maps hanging in item frames as PNGs"`
	Inventories   bool     `opt:"Inventories" flag:"save-inventories" default:"true" desc:"What is in chests you open, and your own inventory"`
	BlockUpdates  bool     `opt:"Block Updates" flag:"block-updates" desc:"Keep blocks the server changes after a chunk loaded"`
	EntityCulling bool     `opt:"Entity Culling" flag:"entity-culling" desc:"Leave out entities that died or despawned (experimental)"`
	ExcludeMobs   []string `opt:"Exclude Mobs" flag:"exclude-mobs" desc:"Entity IDs to leave out, comma separated; * works, e.g. minecraft:bat,hive:*"`
	ChunkRadius   int      `opt:"Chunk Radius" flag:"chunk-radius" desc:"View distance to ask the server for, in chunks (76 if empty)"`
	ScriptPath    string   `opt:"Script" flag:"script" desc:"A JavaScript file that decides what gets saved" type:"file,js"`
	ReplayJoin    bool     `opt:"Watch Replay" flag:"replay-join" desc:"When replaying a .pcap2, wait for Minecraft to join and watch it"`
	RecordUI      bool     `opt:"Record Menus" flag:"record-ui" default:"false" desc:"Rebuild the menus you open and the scoreboards as a script in the world"`
}

// generator is -generator, or flat for the old -void=false.
func (s *WorldSettings) generator() (worldstate.Generator, error) {
	generator, err := worldstate.ParseGenerator(s.Generator)
	if err != nil {
		return "", err
	}
	if !s.Void && generator == worldstate.GeneratorVoid {
		generator = worldstate.GeneratorFlat
	}
	return generator, nil
}

type WorldCMD struct{}

func (WorldCMD) Name() string {
	return "worlds"
}

func (WorldCMD) Description() string {
	return locale.Loc("world_synopsis", nil)
}

func (WorldCMD) Settings() any {
	return new(WorldSettings)
}

func (WorldCMD) Run(ctx context.Context, settings any) error {
	worldSettings := settings.(*WorldSettings)

	var scriptSource string
	if worldSettings.ScriptPath != "" {
		data, err := os.ReadFile(worldSettings.ScriptPath)
		if err != nil {
			return err
		}
		scriptSource = string(data)
	}

	generator, err := worldSettings.generator()
	if err != nil {
		return err
	}

	p, err := proxy.New(ctx, worldSettings.ProxySettings)
	if err != nil {
		return err
	}

	p.AddHandler(worlds.NewWorldsHandler(ctx, worlds.WorldSettings{
		Generator:       generator,
		SaveEntities:    worldSettings.Entities,
		SaveInventories: worldSettings.Inventories,
		SaveImage:       worldSettings.Image,
		ExcludedMobs:    worldSettings.ExcludeMobs,
		ChunkRadius:     int32(worldSettings.ChunkRadius),
		Script:          scriptSource,
		BlockUpdates:    worldSettings.BlockUpdates,
		EntityCulling:   worldSettings.EntityCulling,
		Players:         worldSettings.Players,
		MapArt:          worldSettings.MapArt,
		RecordUI:        worldSettings.RecordUI,
	}))

	// a replay plays the recorded session back by itself: it doesn't need
	// Minecraft to join (unless -replay-join is given, to watch it)
	withClient := true
	if ci := worldSettings.ProxySettings.ConnectInfo; ci != nil && !worldSettings.ReplayJoin {
		if parsed, err := ci.Parsed(); err == nil && parsed.Kind == connectinfo.KindReplay {
			withClient = false
		}
	}
	err = p.Run(ctx, withClient)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	commands.RegisterCommand(&WorldCMD{})
}
