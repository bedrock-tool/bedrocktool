//go:build packs

package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	resourcepackd "github.com/bedrock-tool/bedrocktool/subcommands/resourcepack-d"
	"github.com/bedrock-tool/bedrocktool/utils"
)

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

func (c *ResourcePackCMD) Execute(ctx context.Context, ui utils.UI) error {
	return resourcepackd.Execute_cmd(ctx, c.ServerAddress, c.OnlyKeys, c.SaveEncrypted)
}

func init() {
	utils.RegisterCommand(&ResourcePackCMD{})

}
