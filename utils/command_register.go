package utils

import (
	"fyne.io/fyne/v2/widget"
	"github.com/google/subcommands"
)

var ValidCMDs = make(map[string]Command, 0)

type Command interface {
	subcommands.Command
	SettingsUI() *widget.Form
	MainWindow() error
}

func RegisterCommand(sub Command) {
	subcommands.Register(sub, "")
	ValidCMDs[sub.Name()] = sub
}
