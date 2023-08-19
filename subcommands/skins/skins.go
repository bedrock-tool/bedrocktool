package skins

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft"
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
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	p, _ := proxy.New(ui, !c.NoProxy)
	p.AddHandler(handlers.NewSkinSaver(func(sa handlers.SkinAdd) {
		ui.Message(messages.NewSkin{
			PlayerName: sa.PlayerName,
			Skin:       sa.Skin,
		})
	}))
	p.AddHandler(&proxy.Handler{
		Name: "Skin CMD",
		OnClientConnect: func(conn minecraft.IConn) {
			ui.Message(messages.SetUIState(messages.UIStateConnecting))
		},
		OnServerConnect: func() (cancel bool) {
			ui.Message(messages.SetUIState(messages.UIStateMain))
			return false
		},
	})

	if p.WithClient {
		ui.Message(messages.SetUIState(messages.UIStateConnect))
	} else {
		ui.Message(messages.SetUIState(messages.UIStateConnecting))
	}
	err = p.Run(ctx, address, hostname)
	return err
}

func init() {
	commands.RegisterCommand(&SkinCMD{})
}
