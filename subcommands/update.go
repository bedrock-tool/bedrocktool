// Package subcommands ...
package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"
)

type UpdateCMD struct{}

func (*UpdateCMD) Name() string               { return "update" }
func (*UpdateCMD) Synopsis() string           { return locale.Loc("update_synopsis", nil) }
func (c *UpdateCMD) SetFlags(f *flag.FlagSet) {}

func (c *UpdateCMD) Execute(ctx context.Context, ui ui.UI) error {
	update, err := updater.UpdateAvailable()
	if err != nil {
		return err
	}
	isNew := update.Version != updater.Version
	if !isNew {
		logrus.Info(locale.Loc("no_update", nil))
		return nil
	}
	logrus.Infof(locale.Loc("updating", locale.Strmap{"Version": update.Version}))

	if err := updater.DoUpdate(); err != nil {
		return err
	}

	logrus.Infof(locale.Loc("updated", nil))
	return nil
}

func init() {
	commands.RegisterCommand(&UpdateCMD{})
}
