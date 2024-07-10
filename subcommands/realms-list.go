package subcommands

import (
	"context"
	"flag"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type RealmListCMD struct{}

func (*RealmListCMD) Name() string               { return "list-realms" }
func (*RealmListCMD) Synopsis() string           { return locale.Loc("list_realms_synopsis", nil) }
func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Execute(ctx context.Context) error {
	if !utils.Auth.LoggedIn() {
		err := utils.Auth.Login(ctx, nil)
		if err != nil {
			return err
		}
	}
	realmsClient, err := utils.Auth.Realms()
	if err != nil {
		return err
	}
	realms, err := realmsClient.Realms(ctx)
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
