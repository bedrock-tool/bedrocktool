//go:build nopacks

package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type ResourcePackCMD struct {
	ServerAddress string
	SaveEncrypted bool
	OnlyKeys      bool
}

func (*ResourcePackCMD) Name() string     { return "packs" }
func (*ResourcePackCMD) Synopsis() string { return "NOT COMPILED" }

func (c *ResourcePackCMD) SetFlags(f *flag.FlagSet) {}

func (c *ResourcePackCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis()
}

func (c *ResourcePackCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	logrus.Error("not compiled")
	return 1
}

func init() {
	utils.RegisterCommand(&ResourcePackCMD{})
}
