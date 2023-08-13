package world

import (
	"context"
	"flag"

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
}

func (c *WorldCMD) Execute(ctx context.Context, ui ui.UI) error {
	serverAddress, hostname, err := ui.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, err := proxy.New(ui)
	if err != nil {
		return err
	}

	proxy.AlwaysGetPacks = true
	proxy.AddHandler(worlds.NewWorldsHandler(ui, worlds.WorldSettings{
		VoidGen:         c.EnableVoid,
		WithPacks:       c.Packs,
		SaveEntities:    c.SaveEntities,
		SaveInventories: c.SaveInventories,
		SaveImage:       c.SaveImage,
	}))

	err = proxy.Run(ctx, serverAddress, hostname)
	if err != nil {
		return err
	}
	ui.Message(messages.SetUIState(messages.UIStateFinished))
	return nil
}

func init() {
	commands.RegisterCommand(&WorldCMD{})
}
