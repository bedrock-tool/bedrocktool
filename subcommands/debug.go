package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
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

func (c *DebugProxyCMD) Execute(ctx context.Context, uiHandler messages.Handler) error {
	proxy, err := proxy.New(uiHandler, true)
	if err != nil {
		return err
	}
	utils.Options.Debug = true
	return proxy.Run(ctx, c.ServerAddress)
}

func init() {
	commands.RegisterCommand(&DebugProxyCMD{})
}
