//go:build !nopacks

package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	resourcepackd "github.com/bedrock-tool/bedrocktool/subcommands/resourcepack-d"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

// decrypt using cfb with segmentsize = 1

type ResourcePackCMD struct {
	ServerAddress string
	SaveEncrypted bool
	OnlyKeys      bool
}

func (*ResourcePackCMD) Name() string     { return "packs" }
func (*ResourcePackCMD) Synopsis() string { return locale.Loc("pack_synopsis", nil) }

func (c *ResourcePackCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.BoolVar(&c.SaveEncrypted, "save-encrypted", false, locale.Loc("save_encrypted", nil))
	f.BoolVar(&c.OnlyKeys, "only-keys", false, locale.Loc("only_keys", nil))
}

func (c *ResourcePackCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *ResourcePackCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	err := resourcepackd.Execute_cmd(ctx, c.ServerAddress, c.OnlyKeys, c.SaveEncrypted)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func init() {
	utils.RegisterCommand(&ResourcePackCMD{})

}
