package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type CaptureCMD struct {
	ServerAddress     string
	EnableClientCache bool
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return locale.Loc("capture_synopsis", nil) }
func (c *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
	f.BoolVar(&c.EnableClientCache, "client-cache", true, "Enable Client Cache")
}

func (c *CaptureCMD) Execute(ctx context.Context) error {
	p, err := proxy.New(true, c.EnableClientCache)
	if err != nil {
		return err
	}
	utils.Options.Capture = true

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	return p.Run(ctx, server)
}
func init() {
	commands.RegisterCommand(&CaptureCMD{})
}
