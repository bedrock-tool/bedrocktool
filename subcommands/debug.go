package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type DebugProxyCMD struct {
	ServerAddress string
	ListenAddress string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }
func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.ListenAddress, "listen", "0.0.0.0:19132", "example :19132 or 127.0.0.1:19132")
}

func (c *DebugProxyCMD) Execute(ctx context.Context) error {
	proxy, err := proxy.New(true)
	if err != nil {
		return err
	}
	proxy.ListenAddress = c.ListenAddress
	utils.Options.Debug = true
	return proxy.Run(ctx, c.ServerAddress)
}

func init() {
	commands.RegisterCommand(&DebugProxyCMD{})
}
