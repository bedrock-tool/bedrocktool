package skins

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
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

func (c *SkinCMD) Execute(ctx context.Context) error {
	p, err := proxy.New(!c.NoProxy)
	if err != nil {
		return err
	}

	p.AddHandler(handlers.NewSkinSaver(func(sa handlers.SkinAdd) {
		messages.Router.Handle(&messages.Message{
			Source: "skins",
			Target: "ui",
			Data: messages.NewSkin{
				PlayerName: sa.PlayerName,
				Skin:       sa.Skin,
			},
		})
	}))

	p.AddHandler(&proxy.Handler{
		Name: "Skin CMD",
		ConnectCB: func() bool {
			messages.Router.Handle(&messages.Message{
				Source: "skins",
				Target: "ui",
				Data:   messages.UIStateMain,
			})
			return false
		},
	})

	err = p.Run(ctx, c.ServerAddress)
	return err
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
