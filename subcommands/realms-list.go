package subcommands

import (
	"context"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type RealmListCMD struct{}

func (RealmListCMD) Name() string {
	return "list-realms"
}

func (RealmListCMD) Description() string {
	return locale.Loc("list_realms_synopsis", nil)
}

func (RealmListCMD) Settings() any {
	return nil
}

func (RealmListCMD) Run(ctx context.Context, settings any) error {
	if !auth.Auth.LoggedIn() {
		err := auth.Auth.Login(ctx, &xbox.DeviceTypeAndroid, "")
		if err != nil {
			return err
		}
	}
	account := auth.Auth.Account()
	realms, err := account.Realms().Realms(ctx)
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
