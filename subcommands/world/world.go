package world

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

func init() {
	utils.RegisterCommand(&WorldCMD{})
}

type WorldCMD struct {
	ServerAddress string
	Packs         bool
	EnableVoid    bool
	SaveImage     bool
}

func (*WorldCMD) Name() string     { return "worlds" }
func (*WorldCMD) Synopsis() string { return locale.Loc("world_synopsis", nil) }

func (c *WorldCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.BoolVar(&c.Packs, "packs", false, locale.Loc("save_packs_with_world", nil))
	f.BoolVar(&c.EnableVoid, "void", true, locale.Loc("enable_void", nil))
	f.BoolVar(&c.SaveImage, "image", false, locale.Loc("save_image", nil))
}

func (c *WorldCMD) Execute(ctx context.Context, ui utils.UI) error {
	serverAddress, hostname, err := ui.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}

	proxy.AlwaysGetPacks = true
	proxy.AddHandler(worlds.NewWorldsHandler(ctx, ui, worlds.WorldSettings{
		VoidGen:   c.EnableVoid,
		WithPacks: c.Packs,
		SaveImage: c.SaveImage,
	}))

	ui.Message(messages.SetUIState(messages.UIStateConnect))
	err = proxy.Run(ctx, serverAddress, hostname)
	if err != nil {
		return err
	}
	ui.Message(messages.SetUIState(messages.UIStateFinished))
	return nil
}
