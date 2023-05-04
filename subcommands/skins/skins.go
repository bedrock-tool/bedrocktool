package skins

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
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

func (c *SkinCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, _ := utils.NewProxy()
	proxy.WithClient = !c.NoProxy
	proxy.AddHandler(handlers.NewSkinSaver(func(sa handlers.SkinAdd) {
		ui.Message(messages.NewSkin{
			PlayerName: sa.PlayerName,
			Skin:       sa.Skin,
		})
	}))
	proxy.AddHandler(&utils.ProxyHandler{
		Name: "Skin CMD",
		OnClientConnect: func(conn minecraft.IConn) {
			ui.Message(messages.SetUIState(messages.UIStateConnecting))
		},
		OnServerConnect: func() (cancel bool) {
			ui.Message(messages.SetUIState(messages.UIStateMain))
			return false
		},
	})

	if proxy.WithClient {
		ui.Message(messages.SetUIState(messages.UIStateConnect))
	} else {
		ui.Message(messages.SetUIState(messages.UIStateConnecting))
	}
	err = proxy.Run(ctx, address, hostname)
	return err
}

func init() {
	utils.RegisterCommand(&SkinCMD{})
}
