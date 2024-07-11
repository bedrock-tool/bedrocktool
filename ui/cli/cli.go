package cli

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/google/subcommands"
)

type CLI struct {
	ctx context.Context
}

func (c *CLI) Init() bool {
	messages.Router.AddHandler("ui", c.HandleMessage)
	return true
}

func (c *CLI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	c.ctx = ctx
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
	switch data := msg.Data.(type) {
	case messages.RequestLogin:
		if data.Wait {
			utils.Auth.Login(c.ctx, nil)
		} else {
			go utils.Auth.Login(c.ctx, nil)
		}
	}
	return nil
}
