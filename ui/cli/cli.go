package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
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
		go updater.CheckUpdate()
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
	// "bedrocktool -h" lists the commands; "bedrocktool worlds -h" falls
	// through to the command's own flags
	if subcommandIndex == -1 || os.Args[1+subcommandIndex] == "help" {
		for _, arg := range os.Args {
			if arg == "-h" || arg == "-help" || arg == "--help" || arg == "help" {
				printCommands()
				return nil
			}
		}
	}

	if subcommandIndex == -1 {
		return fmt.Errorf("no command selected")
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
		logrus.Error(locale.Loc("unknown_subcommand", locale.Strmap{"Name": subcommandName}))
		printCommands()
		return nil
	}

	settings, flags, err := commands.ParseArgs(ctx, subcommand, args)
	if errors.Is(err, flag.ErrHelp) {
		return nil // the flag set printed the usage
	}
	if err != nil {
		if flags != nil {
			fmt.Println(locale.Loc("cli_usage_for", locale.Strmap{"Name": subcommandName}))
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
	switch event := event.(type) {
	case *messages.EventStoriesPrompt:
		go func() {
			fmt.Printf("\n%s\n", locale.Loc("cli_stories_text", locale.Strmap{"Realm": event.RealmName}))
			answer, cancelled := utils.UserInput(context.Background(), locale.Loc("cli_stories_prompt", nil), nil)
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "in", "i", "opt in", "yes", "y":
				event.Reply <- messages.StoriesOptIn
			case "out", "o", "opt out", "no", "n":
				if !cancelled {
					event.Reply <- messages.StoriesOptOut
					return
				}
				event.Reply <- messages.StoriesCancel
			default:
				event.Reply <- messages.StoriesCancel
			}
		}()
	}
	return nil
}
