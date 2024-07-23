package world

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type WorldCMD struct {
	ServerAddress   string
	ListenAddress   string
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
	f.StringVar(&c.ListenAddress, "listen", "0.0.0.0:19132", "example :19132 or 127.0.0.1:19132")
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

func (c *WorldCMD) Execute(ctx context.Context) error {
	var script string
	if c.ScriptPath != "" {
		data, err := os.ReadFile(c.ScriptPath)
		if err != nil {
			return err
		}
		script = string(data)
	}

	proxy, err := proxy.New(true)
	if err != nil {
		return err
	}
	proxy.ListenAddress = c.ListenAddress

	proxy.AddHandler(worlds.NewWorldsHandler(worlds.WorldSettings{
		VoidGen:         c.EnableVoid,
		SaveEntities:    c.SaveEntities,
		SaveInventories: c.SaveInventories,
		SaveImage:       c.SaveImage,
		ExcludedMobs:    strings.Split(c.ExcludeMobs, ","),
		StartPaused:     c.StartPaused,
		PreloadReplay:   c.PreloadReplay,
		ChunkRadius:     int32(c.ChunkRadius),
		Script:          script,
	}))

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	err = proxy.Run(ctx, server)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	commands.RegisterCommand(&WorldCMD{})
}
