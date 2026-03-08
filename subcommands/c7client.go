package subcommands

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/handlers/c7client"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type C7ClientSettings struct {
	ProxySettings  proxy.ProxySettings
	ModuleSettings c7client.ModuleSettings
}

type C7ClientCMD struct{}

func (C7ClientCMD) Name() string {
	return "c7"
}

func (C7ClientCMD) Description() string {
	return "C7 CLIENT - Modular utility features (player tracking, etc.)"
}

func (C7ClientCMD) Settings() any {
	return new(C7ClientSettings)
}

func (C7ClientCMD) Run(ctx context.Context, settings any) error {
	c7Settings := settings.(*C7ClientSettings)

	p, err := proxy.New(ctx, c7Settings.ProxySettings)
	if err != nil {
		return err
	}

	// Add the C7 Client handler with configured modules
	p.AddHandler(c7client.NewC7Handler(ctx, c7Settings.ModuleSettings))

	// Add UI state handler
	p.AddHandler(func() *proxy.Handler {
		return &proxy.Handler{
			Name: "C7 UI State",
			OnConnect: func(_ *proxy.Session) error {
				messages.SendEvent(&messages.EventSetUIState{
					State: messages.UIStateMain,
				})
				return nil
			},
		}
	})

	return p.Run(ctx, true)
}

func init() {
	commands.RegisterCommand(&C7ClientCMD{})
}
