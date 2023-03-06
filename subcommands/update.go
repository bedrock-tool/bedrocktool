// Package subcommands ...
package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

type UpdateCMD struct{}

func (*UpdateCMD) Name() string               { return "update" }
func (*UpdateCMD) Synopsis() string           { return locale.Loc("update_synopsis", nil) }
func (c *UpdateCMD) SetFlags(f *flag.FlagSet) {}

func (c *UpdateCMD) Execute(ctx context.Context, ui utils.UI) error {
	newVersion, err := utils.Updater.UpdateAvailable()
	if err != nil {
		return err
	}
	if newVersion == "" {
		logrus.Info(locale.Loc("no_update", nil))
		return nil
	}
	logrus.Infof(locale.Loc("updating", locale.Strmap{"Version": newVersion}))

	if err := utils.Updater.Update(); err != nil {
		return err
	}

	logrus.Infof(locale.Loc("updated", nil))
	return nil
}

func init() {
	utils.RegisterCommand(&UpdateCMD{})
}
