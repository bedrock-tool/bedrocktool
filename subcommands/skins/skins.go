package skins

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type SkinCMD struct {
	ServerAddress string
	Filter        string
	NoProxy       bool
}

func (*SkinCMD) Name() string     { return "skins" }
func (*SkinCMD) Synopsis() string { return locale.Loc("skins_synopsis", nil) }

func (c *SkinCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.Filter, "filter", "", locale.Loc("name_prefix", nil))
	f.BoolVar(&c.NoProxy, "no-proxy", false, "use headless version")
}

func (c *SkinCMD) Execute(ctx context.Context, ui ui.UI) error {
	p, err := proxy.New(ui, !c.NoProxy)
	if err != nil {
		return err
	}

	p.AddHandler(handlers.NewSkinSaver(func(sa handlers.SkinAdd) {
		ui.Message(messages.NewSkin{
			PlayerName: sa.PlayerName,
			Skin:       sa.Skin,
		})
	}))

	p.AddHandler(&proxy.Handler{
		Name: "Skin CMD",
		ConnectCB: func() bool {
			ui.Message(messages.SetUIState(messages.UIStateMain))
			return false
		},
	})

	err = p.Run(ctx, c.ServerAddress)
	return err
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
