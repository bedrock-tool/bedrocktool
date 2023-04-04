package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	seconduser "github.com/bedrock-tool/bedrocktool/utils/handlers/second-user"
)

type DebugProxyCMD struct {
	ServerAddress string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }
func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
}

func (c *DebugProxyCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	utils.Options.Debug = true

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}
	proxy.AddHandler(seconduser.NewSecondUser())
	return proxy.Run(ctx, address, hostname)
}

func init() {
	utils.RegisterCommand(&DebugProxyCMD{})
}
