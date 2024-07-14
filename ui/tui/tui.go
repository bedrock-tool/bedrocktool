package tui

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type TUI struct {
	ctx context.Context
}

var _ ui.UI = &TUI{}

func (c *TUI) Init() bool {
	messages.Router.AddHandler("ui", c.HandleMessage)
	return true
}

func (c *TUI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	c.ctx = ctx
	utils.Auth.SetHandler(nil)
	isDebug := updater.Version == ""
	if !isDebug {
		go updater.UpdateCheck(c)
	}
	select {
	case <-ctx.Done():
		return nil
	default:
		fmt.Println(locale.Loc("available_commands", nil))
		for name, cmd := range commands.Registered {
			fmt.Printf("\t%s\t%s\n\r", name, cmd.Synopsis())
		}
		fmt.Println(locale.Loc("use_to_run_command", nil))

		cmd, cancelled := utils.UserInput(ctx, locale.Loc("input_command", nil), func(s string) bool {
			for k := range commands.Registered {
				if s == k {
					return true
				}
			}
			return false
		})
		if cancelled {
			cancel(errors.New("cancelled input"))
			return nil
		}
		_cmd := strings.Split(cmd, " ")
		os.Args = append(os.Args, _cmd...)
	}
	flag.Parse()
	subcommands.Execute(ctx, c)

	logrus.Info(locale.Loc("enter_to_exit", nil))
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	return nil
}

func (c *TUI) HandleMessage(msg *messages.Message) *messages.Message {
	switch msg := msg.Data.(type) {
	case messages.RequestLogin:
		if msg.Wait {
			utils.Auth.Login(c.ctx)
		} else {
			go utils.Auth.Login(c.ctx)
		}
	case *messages.ServerInput:
		_ = msg
		var cancelled bool
		// utils.ValidateServerInput
		server, cancelled := utils.UserInput(c.ctx, locale.Loc("enter_server", nil), nil)
		if cancelled {
			return &messages.Message{
				Source: "gui",
				Data:   messages.Error(context.Canceled),
			}
		}

		ret, err := utils.ParseServer(c.ctx, server)
		if err != nil {
			return &messages.Message{
				Source: "gui",
				Data:   messages.Error(err),
			}
		}

		return &messages.Message{
			Source: "gui",
			Data:   ret,
		}
	}
	return nil
}
