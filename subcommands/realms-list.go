package subcommands

import (
	"context"
	"flag"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type RealmListCMD struct{}

func (*RealmListCMD) Name() string               { return "list-realms" }
func (*RealmListCMD) Synopsis() string           { return locale.Loc("list_realms_synopsis", nil) }
func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Execute(ctx context.Context, ui ui.UI) error {
	realms, err := utils.GetRealmsAPI().Realms(ctx)
	if err != nil {
		return err
	}
	for _, realm := range realms {
		fmt.Println(locale.Loc("realm_list_line", locale.Strmap{"Name": realm.Name, "Id": realm.ID}))
	}
	return nil
}

func init() {
	commands.RegisterCommand(&RealmListCMD{})
}
