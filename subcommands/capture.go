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

type CaptureCMD struct {
	ServerAddress string
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return locale.Loc("capture_synopsis", nil) }
func (c *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
}

func (c *CaptureCMD) Execute(ctx context.Context, ui ui.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	p, err := proxy.New(ui, true)
	if err != nil {
		return err
	}
	utils.Options.Capture = true
	return p.Run(ctx, address, hostname)
}
func init() {
	commands.RegisterCommand(func() commands.Command {
		return new(CaptureCMD)
	})
}
