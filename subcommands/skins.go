package subcommands

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type SkinSettings struct {
	ProxySettings proxy.ProxySettings

	Filter  string `opt:"Name Filter" flag:"filter"`
	NoProxy bool   `opt:"No Proxy" flag:"no-proxy"`
}

type SkinCMD struct {
	ServerAddress     string
	ListenAddress     string
	Filter            string
	NoProxy           bool
	EnableClientCache bool
}

func (SkinCMD) Name() string {
	return "skins"
}

func (SkinCMD) Description() string {
	return locale.Loc("skins_synopsis", nil)
}

func (SkinCMD) Settings() any {
	return new(SkinSettings)
}

func (SkinCMD) Run(ctx context.Context, settings any) error {
	skinSettings := settings.(*SkinSettings)

	p, err := proxy.New(ctx, skinSettings.ProxySettings)
	if err != nil {
		return err
	}

	p.AddHandler(handlers.NewSkinSaver(func(sa handlers.SkinAdd) {
		messages.SendEvent(&messages.EventPlayerSkin{
			PlayerName: sa.PlayerName,
			Skin:       *sa.Skin,
		})
	}))

	p.AddHandler(func() *proxy.Handler {
		return &proxy.Handler{
			Name: "Skin CMD",
			OnConnect: func(_ *proxy.Session) bool {
				messages.SendEvent(&messages.EventSetUIState{
					State: messages.UIStateMain,
				})
				return false
			},
		}
	})

	return p.Run(!skinSettings.NoProxy)
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
