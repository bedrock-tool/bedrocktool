package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"

	"github.com/google/subcommands"
)

type UpdateCMD struct{}

func (*UpdateCMD) Name() string     { return "update" }
func (*UpdateCMD) Synopsis() string { return "self updates to latest version" }

func (c *UpdateCMD) SetFlags(f *flag.FlagSet) {}

func (c *UpdateCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *UpdateCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	newVersion, err := utils.Updater.UpdateAvailable()
	if err != nil {
		logrus.Error(err)
		return 1
	}
	if newVersion == "" {
		logrus.Info("No Updates available.")
	}
	logrus.Infof("Updating to %s", newVersion)

	if err := utils.Updater.Update(); err != nil {
		logrus.Error(err)
		return 1
	}

	logrus.Infof("Updated!")
	return 0
}

func init() {
	utils.RegisterCommand(&UpdateCMD{})
}
