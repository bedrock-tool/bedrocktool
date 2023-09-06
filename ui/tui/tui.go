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
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

type TUI struct{}

var _ ui.UI = &TUI{}

func (c *TUI) Init() bool {
	return true
}

func (c *TUI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		fmt.Println(locale.Loc("available_commands", nil))
		for name, cmd := range commands.Registered {
			fmt.Printf("\t%s\t%s\n", name, cmd.Synopsis())
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

func (c *TUI) ServerInput(ctx context.Context, server string) (string, string, error) {
	return utils.ServerInput(ctx, server)
}

func (c *TUI) Message(data interface{}) messages.Response {
	return messages.Response{
		Ok:   false,
		Data: nil,
	}
}
