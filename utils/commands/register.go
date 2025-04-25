package commands

import (
	"context"
)

var Registered = map[string]Command{}

type Command interface {
	Name() string
	Description() string
	Settings() any
	Run(ctx context.Context, settings any) error
}

func RegisterCommand(sub Command) {
	Registered[sub.Name()] = sub
}
