package subcommands

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"
)

type UpdateCMD struct{}

func (UpdateCMD) Name() string {
	return "update"
}
func (UpdateCMD) Description() string {
	return locale.Loc("update_synopsis", nil)
}
func (UpdateCMD) Settings() any {
	return nil
}

func (c *UpdateCMD) Run(ctx context.Context, settings any) error {
	update, err := updater.UpdateAvailable()
	if err != nil {
		return err
	}
	isNew := update.Version != utils.Version
	if !isNew {
		logrus.Info(locale.Loc("no_update", nil))
		return nil
	}
	logrus.Info(locale.Loc("updating", locale.Strmap{"Version": update.Version}))

	if err := updater.DoUpdate(); err != nil {
		return err
	}

	logrus.Info(locale.Loc("updated", nil))
	return nil
}

func init() {
	commands.RegisterCommand(&UpdateCMD{})
}
