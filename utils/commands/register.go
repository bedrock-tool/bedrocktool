package commands

import (
	"context"
	"flag"
)

var Registered = map[string]Command{}

type Command interface {
	Name() string
	Synopsis() string
	SetFlags(f *flag.FlagSet)
	Execute(ctx context.Context) error
}

func RegisterCommand(sub Command) {
	Registered[sub.Name()] = sub
}
