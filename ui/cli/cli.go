package cli

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/subcommands"
)

type CLI struct{}

func (c *CLI) Init() bool {
	return true
}

func (c *CLI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	flag.Parse()
	subcommands.Execute(ctx, c)
	cancel(nil)
	return nil
}

func (c *CLI) ServerInput(ctx context.Context, server string) (string, string, error) {
	return utils.ServerInput(ctx, server)
}

func (c *CLI) Message(data interface{}) messages.Response {
	return messages.Response{
		Ok:   false,
		Data: nil,
	}
}
