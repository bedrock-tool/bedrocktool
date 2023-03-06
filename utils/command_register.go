package utils

import (
	"github.com/google/subcommands"
)

var ValidCMDs = make(map[string]Command, 0)

type Command interface {
	subcommands.Command
}

func RegisterCommand(sub Command) {
	subcommands.Register(sub, "")
	ValidCMDs[sub.Name()] = sub
}
