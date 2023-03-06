package subcommands

import (
	"context"
	"flag"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type DebugProxyCMD struct {
	ServerAddress string
	Filter        string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }
func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.Filter, "filter", "", locale.Loc("packet_filter", nil))
}

func (c *DebugProxyCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, _, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	utils.Options.Debug = true

	filters := strings.Split(c.Filter, ",")
	if len(filters) > 0 {
		for _, v := range filters {
			if len(v) == 0 {
				continue
			}
			if string(v[0]) == "*" {
				v = v[1:]
			}
			v = strings.TrimPrefix(v, "packet.")
			v = "packet." + v
			utils.ExtraVerbose = append(utils.ExtraVerbose, v)
		}
	}

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}
	err = proxy.Run(ctx, address)
	return err
}

func init() {
	utils.RegisterCommand(&DebugProxyCMD{})
}
