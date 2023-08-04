package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type DebugProxyCMD struct {
	ServerAddress string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }
func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
}

func (c *DebugProxyCMD) Execute(ctx context.Context, ui ui.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	utils.Options.Debug = true

	proxy, err := proxy.New(ui)
	if err != nil {
		return err
	}
	return proxy.Run(ctx, address, hostname)
}

func init() {
	commands.RegisterCommand(&DebugProxyCMD{})
}
