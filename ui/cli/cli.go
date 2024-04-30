package cli

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/google/subcommands"
)

type CLI struct{}

func (c *CLI) Init() bool {
	return true
}

func (c *CLI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	isDebug := updater.Version == ""
	if !isDebug {
		go updater.UpdateCheck(c)
	}
	flag.Parse()
	subcommands.Execute(ctx, c)
	cancel(nil)
	return nil
}

func (c *CLI) HandleMessage(msg *messages.Message) *messages.Message {
	return nil
}
