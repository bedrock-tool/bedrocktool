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
	ServerAddress     string
	ListenAddress     string
	EnableClientCache bool
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }
func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.ListenAddress, "listen", "0.0.0.0:19132", "example :19132 or 127.0.0.1:19132")
	f.BoolVar(&c.EnableClientCache, "client-cache", true, "Enable Client Cache")
}

func (c *DebugProxyCMD) Execute(ctx context.Context) error {
	proxyContext, err := proxy.New(true, c.EnableClientCache)
	if err != nil {
		return err
	}
	proxyContext.ListenAddress = c.ListenAddress
	utils.Options.Debug = true

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	return proxyContext.Run(ctx, server)
}

func init() {
	commands.RegisterCommand(&DebugProxyCMD{})
}
