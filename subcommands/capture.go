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
	ListenAddress     string
	EnableClientCache bool
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return locale.Loc("capture_synopsis", nil) }
func (c *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
	f.StringVar(&c.ListenAddress, "listen", "0.0.0.0:19132", "example :19132 or 127.0.0.1:19132")
	f.BoolVar(&c.EnableClientCache, "client-cache", true, "Enable Client Cache")
}

func (c *CaptureCMD) Execute(ctx context.Context) error {
	p, err := proxy.New(true, c.EnableClientCache)
	if err != nil {
		return err
	}
	p.ListenAddress = c.ListenAddress
	utils.Options.Capture = true

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	return p.Run(ctx, server)
}
func init() {
	commands.RegisterCommand(&CaptureCMD{})
}
