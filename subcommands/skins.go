package subcommands

import (
	"context"
	"regexp"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type SkinSettings struct {
	ProxySettings proxy.ProxySettings

	Filter      string `opt:"Name Regex (save if it matches)" flag:"filter"`
	NoProxy     bool   `opt:"No Proxy" flag:"no-proxy"`
	TextureOnly bool   `opt:"Texture Only" flag:"texture-only"`
	Timestamped bool   `opt:"Timestamped" flag:"timestamped" default:"true"`
}

type SkinCMD struct{}

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

	handleSkin := func(sa handlers.SkinAdd) {
		messages.SendEvent(&messages.EventPlayerSkin{
			PlayerName: sa.PlayerName,
			Skin:       *sa.Skin,
		})
	}

	var playerNameFilter *regexp.Regexp
	if skinSettings.Filter != "" {
		playerNameFilter, err = regexp.Compile(skinSettings.Filter)
		if err != nil {
			return err
		}
	}

	p.AddHandler(handlers.NewSkinSaver(handleSkin, playerNameFilter, skinSettings.TextureOnly, skinSettings.Timestamped))

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

	return p.Run(ctx, !skinSettings.NoProxy)
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
