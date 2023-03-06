package utils

import (
	"context"
	"flag"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

var ValidCMDs = make(map[string]Command, 0)

type Command interface {
	Name() string
	Synopsis() string
	SetFlags(f *flag.FlagSet)
	Execute(ctx context.Context, ui UI) error
}

type cmdWrap struct {
	subcommands.Command

	cmd Command
}

func (c *cmdWrap) Name() string             { return c.cmd.Name() }
func (c *cmdWrap) Synopsis() string         { return c.cmd.Synopsis() }
func (c *cmdWrap) SetFlags(f *flag.FlagSet) { c.cmd.SetFlags(f) }
func (c *cmdWrap) Usage() string            { return c.Name() + ": " + c.Synopsis() }
func (c *cmdWrap) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	err := c.cmd.Execute(ctx, currentUI)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func RegisterCommand(sub Command) {
	subcommands.Register(&cmdWrap{cmd: sub}, "")
	ValidCMDs[sub.Name()] = sub
}
