package commands

import (
	"context"
	"flag"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

var Registered = map[string]Command{}

type Command interface {
	Name() string
	Synopsis() string
	SetFlags(f *flag.FlagSet)
	Execute(ctx context.Context) error
}

type cmdWrap struct {
	subcommands.Command

	cmd Command
}

func (c *cmdWrap) Name() string             { return c.cmd.Name() }
func (c *cmdWrap) Synopsis() string         { return c.cmd.Synopsis() + "\n" }
func (c *cmdWrap) SetFlags(f *flag.FlagSet) { c.cmd.SetFlags(f) }
func (c *cmdWrap) Usage() string            { return c.Name() + ": " + c.Synopsis() }
func (c *cmdWrap) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	err := c.cmd.Execute(ctx)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func RegisterCommand(sub Command) {
	subcommands.Register(&cmdWrap{cmd: sub}, "")
	Registered[sub.Name()] = sub
}
