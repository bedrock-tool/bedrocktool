package subcommands

import (
	"context"
	"flag"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
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

func (c *DebugProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *DebugProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, _, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		logrus.Error(err)
		return 1
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
		logrus.Error(err)
		return 1
	}
	if err := proxy.Run(ctx, address); err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func init() {
	utils.RegisterCommand(&DebugProxyCMD{})
}
