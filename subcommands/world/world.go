package world

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type WorldCMD struct {
	ServerAddress   string
	Packs           bool
	EnableVoid      bool
	SaveEntities    bool
	SaveInventories bool
	SaveImage       bool
	ExcludeMobs     string
	StartPaused     bool
	PreloadReplay   string
	ChunkRadius     int
	ScriptPath      string
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return locale.Loc("world_synopsis", nil) }

func (c *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.BoolVar(&c.Packs, "packs", false, locale.Loc("save_packs_with_world", nil))
	f.BoolVar(&c.EnableVoid, "void", true, locale.Loc("enable_void", nil))
	f.BoolVar(&c.SaveImage, "image", false, locale.Loc("save_image", nil))
	f.BoolVar(&c.SaveEntities, "save-entities", true, "Save Entities")
	f.BoolVar(&c.SaveInventories, "save-inventories", true, "Save Inventories")
	f.StringVar(&c.ExcludeMobs, "exclude-mobs", "", "list of mobs to exclude seperated by comma")
	f.BoolVar(&c.StartPaused, "start-paused", false, "pause the capturing on startup (can be restarted using /start-capture ingame)")
	f.StringVar(&c.PreloadReplay, "preload-replay", "", "preload from a replay")
	f.IntVar(&c.ChunkRadius, "chunk-radius", 0, "the max chunk radius to force")
	f.StringVar(&c.ScriptPath, "script", "", "path to script to use")
}

func (c *WorldCMD) Execute(ctx context.Context, ui ui.UI) error {
	var script string
	if c.ScriptPath != "" {
		data, err := os.ReadFile(c.ScriptPath)
		if err != nil {
			return err
		}
		script = string(data)
	}

	proxy, err := proxy.New(ui, true)
	if err != nil {
		return err
	}

	proxy.AddHandler(worlds.NewWorldsHandler(ui, worlds.WorldSettings{
		VoidGen:         c.EnableVoid,
		WithPacks:       c.Packs,
		SaveEntities:    c.SaveEntities,
		SaveInventories: c.SaveInventories,
		SaveImage:       c.SaveImage,
		ExcludedMobs:    strings.Split(c.ExcludeMobs, ","),
		StartPaused:     c.StartPaused,
		PreloadReplay:   c.PreloadReplay,
		ChunkRadius:     int32(c.ChunkRadius),
		Script:          script,
	}))

	err = proxy.Run(ctx, c.ServerAddress)
	if err != nil {
		return err
	}
	ui.Message(messages.SetUIState(messages.UIStateFinished))
	return nil
}

func init() {
	commands.RegisterCommand(&WorldCMD{})
}
