package subcommands

import (
	"context"
	"os"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type WorldSettings struct {
	ProxySettings   proxy.ProxySettings
	Void            bool     `opt:"Void Generator" flag:"void" default:"true" desc:"locale.enable_void"`
	Image           bool     `opt:"Image" flag:"image" desc:"locale.save_image"`
	Entities        bool     `opt:"Entities" flag:"save-entities" default:"true" desc:"Save Entities"`
	Inventories     bool     `opt:"Inventories" flag:"save-inventories" default:"true" desc:"Save Inventories"`
	BlockUpdates    bool     `opt:"Block Updates" flag:"block-updates" desc:"Block updates"`
	ExcludeMobs     []string `opt:"Exclude Mobs" flag:"exclude-mobs" desc:"list of mobs to exclude seperated by comma"`
	StartPaused     bool     `opt:"Start Paused" flag:"start-paused" desc:"pause the capturing on startup (can be restarted using /start-capture ingame)"`
	PreloadedReplay string   `opt:"Preload Replay" flag:"preload-replay" desc:"preload from a replay" type:"file,pcap2"`
	ChunkRadius     int      `opt:"Chunk Radius" flag:"chunk-radius" desc:"the max chunk radius to force"`
	ScriptPath      string   `opt:"Script Path" flag:"script" desc:"path to script to use" type:"file,js"`
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

	p, err := proxy.New(ctx, worldSettings.ProxySettings)
	if err != nil {
		return err
	}

	p.AddHandler(worlds.NewWorldsHandler(ctx, worlds.WorldSettings{
		VoidGen:         worldSettings.Void,
		SaveEntities:    worldSettings.Entities,
		SaveInventories: worldSettings.Inventories,
		SaveImage:       worldSettings.Image,
		ExcludedMobs:    worldSettings.ExcludeMobs,
		StartPaused:     worldSettings.StartPaused,
		PreloadReplay:   worldSettings.PreloadedReplay,
		ChunkRadius:     int32(worldSettings.ChunkRadius),
		Script:          scriptSource,
		BlockUpdates:    worldSettings.BlockUpdates,
		//Players:         true,
	}))

	err = p.Run(ctx, true)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	commands.RegisterCommand(&WorldCMD{})
}
