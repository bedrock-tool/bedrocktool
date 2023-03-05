package subcommands

import (
	"context"
	"flag"
	"strings"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type DebugProxyCMD struct {
	serverAddress      string
	filter             string
	pathCustomUserData string
}

func (*DebugProxyCMD) Name() string     { return "debug-proxy" }
func (*DebugProxyCMD) Synopsis() string { return locale.Loc("debug_proxy_synopsis", nil) }

func (c *DebugProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.serverAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.filter, "filter", "", locale.Loc("packet_filter", nil))
	f.StringVar(&c.pathCustomUserData, "userdata", "", locale.Loc("custom_user_data", nil))
}

func (c *DebugProxyCMD) SettingsUI() *widget.Form {
	return widget.NewForm(
		widget.NewFormItem(
			"serverAddress", widget.NewEntryWithData(binding.BindString(&c.serverAddress)),
		), widget.NewFormItem(
			"filter", widget.NewEntryWithData(binding.BindString(&c.filter)),
		), widget.NewFormItem(
			"pathCustomUserData", widget.NewEntryWithData(binding.BindString(&c.pathCustomUserData)),
		),
	)
}

func (c *DebugProxyCMD) MainWindow() error {
	return nil
}

func (c *DebugProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *DebugProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, _, err := utils.ServerInput(ctx, c.serverAddress)
	if err != nil {
		logrus.Error(err)
		return 1
	}

	utils.Options.Debug = true

	filters := strings.Split(c.filter, ",")
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

	proxy, err := utils.NewProxy(c.pathCustomUserData)
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
