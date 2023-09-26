package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type RealmAddressCMD struct {
	realm string
}

func (*RealmAddressCMD) Name() string     { return "realm-address" }
func (*RealmAddressCMD) Synopsis() string { return "gets realms address" }
func (c *RealmAddressCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.realm, "realm", "", "realm name <name:id> or just name")
}

func (c *RealmAddressCMD) Execute(ctx context.Context, ui ui.UI) error {
	address, _, err := utils.ServerInput(ctx, "realm:"+c.realm)
	if err != nil {
		return err
	}

	logrus.Infof("Address: %s", address)
	return nil
}

func init() {
	commands.RegisterCommand(&RealmAddressCMD{})
}
