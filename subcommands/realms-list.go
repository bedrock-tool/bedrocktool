package subcommands

import (
	"context"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/xbox"
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
	if !utils.Auth.LoggedIn() {
		err := utils.Auth.Login(ctx, &xbox.DeviceTypeAndroid)
		if err != nil {
			return err
		}
	}
	realms, err := utils.Auth.Realms().Realms(ctx)
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
