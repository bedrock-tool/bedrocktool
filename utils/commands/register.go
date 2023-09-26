package commands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

var Registered = map[string]Command{}

type Command interface {
	Name() string
	Synopsis() string
	SetFlags(f *flag.FlagSet)
	Execute(ctx context.Context, ui ui.UI) error
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
	if len(args) != 1 {
		panic("invalid args")
	}
	err := c.cmd.Execute(ctx, args[0].(ui.UI))
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
