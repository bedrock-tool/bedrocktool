package utils

import "github.com/google/subcommands"

var ValidCMDs = make(map[string]string, 0)

func RegisterCommand(sub subcommands.Command) {
	subcommands.Register(sub, "")
	ValidCMDs[sub.Name()] = sub.Synopsis()
}
