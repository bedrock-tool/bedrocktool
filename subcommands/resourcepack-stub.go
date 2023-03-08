//go:build !packs

package subcommands

import (
	"context"
	"errors"
	"flag"

	"github.com/bedrock-tool/bedrocktool/utils"
)

type ResourcePackCMD struct {
	ServerAddress string
	SaveEncrypted bool
	OnlyKeys      bool
}

func (*ResourcePackCMD) Name() string             { return "packs" }
func (*ResourcePackCMD) Synopsis() string         { return "NOT COMPILED" }
func (*ResourcePackCMD) SetFlags(f *flag.FlagSet) {}
func (*ResourcePackCMD) Execute(ctx context.Context, ui utils.UI) error {
	return errors.New("not compiled")
}

func init() {
	utils.RegisterCommand(&ResourcePackCMD{})
}
