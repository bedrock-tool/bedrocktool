package skins

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type SkinCMD struct {
	ServerAddress     string
	ListenAddress     string
	Filter            string
	NoProxy           bool
	EnableClientCache bool
}

func (*SkinCMD) Name() string     { return "skins" }
func (*SkinCMD) Synopsis() string { return locale.Loc("skins_synopsis", nil) }

func (c *SkinCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.ListenAddress, "listen", "0.0.0.0:19132", "example :19132 or 127.0.0.1:19132")
	f.StringVar(&c.Filter, "filter", "", locale.Loc("name_prefix", nil))
	f.BoolVar(&c.NoProxy, "no-proxy", false, "use headless version")
	f.BoolVar(&c.EnableClientCache, "client-cache", true, "Enable Client Cache")
}

func (c *SkinCMD) Execute(ctx context.Context) error {
	p, err := proxy.New(ctx, !c.NoProxy, c.EnableClientCache)
	if err != nil {
		return err
	}
	p.ListenAddress = c.ListenAddress

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

	p.AddHandler(func() *proxy.Handler {
		return &proxy.Handler{
			Name: "Skin CMD",
			OnConnect: func(_ *proxy.Session) bool {
				messages.Router.Handle(&messages.Message{
					Source: "skins",
					Target: "ui",
					Data:   messages.UIStateMain,
				})
				return false
			},
		}
	})

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	err = p.Run(server)
	return err
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
