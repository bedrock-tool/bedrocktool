package subcommands

import (
	"context"
	"flag"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type DebugProxyCMD struct {
	Address string
	filter  string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return "verbose debug packets" }

func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.Address, "address", "", "remote server address")
	f.StringVar(&c.filter, "filter", "", "packets to not show")
}

func (c *DebugProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + utils.SERVER_ADDRESS_HELP
}

func (c *DebugProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, _, err := utils.ServerInput(c.Address)
	if err != nil {
		logrus.Error(err)
		return 1
	}

	utils.G_debug = true

	filters := strings.Split(c.filter, ",")
	if len(filters) > 0 {
		for _, v := range filters {
			if string(v[0]) == "*" {
				v = v[1:]
			}
			v = strings.TrimPrefix(v, "packet.")
			v = "packet." + v
			utils.ExtraVerbose = append(utils.ExtraVerbose, v)
		}
	}

	proxy := utils.NewProxy(logrus.StandardLogger())
	if err := proxy.Run(ctx, address); err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func init() {
	utils.RegisterCommand(&DebugProxyCMD{})
}
