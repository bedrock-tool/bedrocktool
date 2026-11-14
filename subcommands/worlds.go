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
	Generator     string   `opt:"locale.opt_worlds_generator" flag:"generator" default:"void" choices:"void,flat,normal" desc:"locale.desc_worlds_generator"`
	Void          bool     `flag:"void" default:"true" hidden:"true" desc:"locale.desc_worlds_void_old"`
	Image         bool     `opt:"locale.opt_worlds_image" flag:"image" desc:"locale.save_image"`
	Entities      bool     `opt:"locale.opt_worlds_save_entities" flag:"save-entities" default:"true" desc:"locale.desc_worlds_save_entities"`
	Players       bool     `opt:"locale.opt_worlds_save_players" flag:"save-players" default:"false" desc:"locale.desc_worlds_save_players"`
	MapArt        bool     `opt:"locale.opt_worlds_map_art" flag:"map-art" default:"false" desc:"locale.desc_worlds_map_art"`
	Inventories   bool     `opt:"locale.opt_worlds_save_inventories" flag:"save-inventories" default:"true" desc:"locale.desc_worlds_save_inventories"`
	BlockUpdates  bool     `opt:"locale.opt_worlds_block_updates" flag:"block-updates" desc:"locale.desc_worlds_block_updates"`
	EntityCulling bool     `opt:"locale.opt_worlds_entity_culling" flag:"entity-culling" desc:"locale.desc_worlds_entity_culling"`
	ExcludeMobs   []string `opt:"locale.opt_worlds_exclude_mobs" flag:"exclude-mobs" desc:"locale.desc_worlds_exclude_mobs"`
	ChunkRadius   int      `opt:"locale.opt_worlds_chunk_radius" flag:"chunk-radius" desc:"locale.desc_worlds_chunk_radius"`
	ScriptPath    string   `opt:"locale.opt_worlds_script" flag:"script" desc:"locale.desc_worlds_script" type:"file,js"`
	ReplayJoin    bool     `opt:"locale.opt_worlds_replay_join" flag:"replay-join" desc:"locale.desc_worlds_replay_join"`
	RecordUI      bool     `opt:"locale.opt_worlds_record_ui" flag:"record-ui" default:"false" desc:"locale.desc_worlds_record_ui"`
	Merge         bool     `opt:"locale.opt_worlds_merge_dimensions" flag:"merge-dimensions" default:"false" desc:"locale.desc_worlds_merge_dimensions"`
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
		MergeDimensions: worldSettings.Merge,
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
