package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"
)

type CLI struct {
	IsInteractive bool
}

func (c *CLI) Init() error {
	messages.SetEventHandler(c.eventHandler)
	return nil
}

func printCommands() {
	fmt.Println(locale.Loc("available_commands", nil))
	for name, cmd := range commands.Registered {
		fmt.Printf("\t%s\t%s\n\r", name, cmd.Description())
	}
}

func (c *CLI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	auth.Auth.SetHandler(nil)
	if !utils.IsDebug() {
		go updater.UpdateCheck(c)
	}

	if c.IsInteractive {
		select {
		case <-ctx.Done():
			return nil
		default:
			printCommands()
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
	}

	var subcommandIndex int = -1
	for i := range len(os.Args[1:]) {
		arg := os.Args[1+i]
		if len(arg) == 0 {
			continue
		}
		if arg[0] == '-' {
			if !strings.ContainsRune(arg, '=') {
				i++
			}
			continue
		}
		subcommandIndex = i
		break
	}
	if subcommandIndex == -1 {
		return fmt.Errorf("no command selected")
	}

	for _, arg := range os.Args {
		if arg == "-h" || arg == "-help" || arg == "help" {
			printCommands()
			return nil
		}
	}

	var args []string
	for i, arg := range os.Args[1:] {
		if subcommandIndex == i {
			continue
		}
		args = append(args, arg)
	}
	subcommandName := os.Args[1+subcommandIndex]

	subcommand, ok := commands.Registered[subcommandName]
	if !ok {
		logrus.Errorf("%s is not a known subcommand", subcommandName)
		printCommands()
		return nil
	}

	settings, flags, err := commands.ParseArgs(ctx, subcommand, args)
	if err != nil {
		if flags != nil {
			fmt.Printf("Usage for %s:\n", subcommandName)
			flags.PrintDefaults()
		}
		return err
	}

	err = subcommand.Run(ctx, settings)
	if err != nil {
		return err
	}
	cancel(nil)

	if c.IsInteractive {
		logrus.Info(locale.Loc("enter_to_exit", nil))
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
	}

	return nil
}

func (c *CLI) eventHandler(event any) error {
	return nil
}
