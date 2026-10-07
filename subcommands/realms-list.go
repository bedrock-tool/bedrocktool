package subcommands

import (
	"context"
	"github.com/sirupsen/logrus"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
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
		err := auth.Auth.Login(ctx, nil, "")
		if err != nil {
			return err
		}
	}
	realmsClient, err := auth.Auth.Account().Realms(ctx)
	if err != nil {
		return err
	}
	realms, err := realmsClient.Realms(ctx)
	if err != nil {
		return err
	}
	logrus.Infof("Available Realms: %d", len(realms))
	for _, realm := range realms {
		logrus.Infof("Realm name=%q ID=%d", realm.Name, realm.ID)
	}
	return nil
}

func init() {
	commands.RegisterCommand(&RealmListCMD{})
}
