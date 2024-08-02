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
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/bedrock-tool/bedrocktool/utils/xbox"
	"github.com/sirupsen/logrus"
)

type CLI struct {
	ctx context.Context
}

func (c *CLI) Init() bool {
	messages.Router.AddHandler("ui", c.HandleMessage)
	return true
}

func printCommands() {
	fmt.Println(locale.Loc("available_commands", nil))
	for name, cmd := range commands.Registered {
		fmt.Printf("\t%s\t%s\n\r", name, cmd.Synopsis())
	}
}

func (c *CLI) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	c.ctx = ctx
	utils.Auth.SetHandler(nil)
	isDebug := updater.Version == ""
	if !isDebug {
		go updater.UpdateCheck(c)
	}

	if utils.Options.IsInteractive {
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

	err := flag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("Available flags: ")
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			fmt.Print("-", f.Name, " ")
		})
		fmt.Print("\n")
		printCommands()
		return err
	}

	subcommandArgs := flag.Args()
	subcommandName := subcommandArgs[0]
	subcommand, ok := commands.Registered[subcommandName]
	if !ok {
		logrus.Errorf("%s is not a known subcommand", subcommandName)
		printCommands()
		return nil
	}
	f := flag.NewFlagSet(subcommandName, flag.ContinueOnError)
	subcommand.SetFlags(f)
	err = f.Parse(subcommandArgs[1:])
	if err != nil {
		fmt.Printf("Available subcommand flags:\n")
		f.VisitAll(func(f *flag.Flag) {
			fmt.Print("\t-", f.Name, "\n")
		})
		return err
	}

	addressFlag := f.Lookup("address")
	var serverAddress string
	if addressFlag != nil {
		if addressFlag.Value.String() != "" {
			serverAddress = addressFlag.Value.String()
		} else {
			var cancelled bool
			serverAddress, cancelled = utils.UserInput(ctx, locale.Loc("enter_server", nil), nil)
			if cancelled {
				return nil
			}
		}
	}
	if serverAddress != "" {
		connectInfo, err := utils.ParseServer(ctx, serverAddress)
		if err != nil {
			return err
		}
		ctx = context.WithValue(ctx, utils.ConnectInfoKey, connectInfo)
	}

	err = subcommand.Execute(ctx)
	if err != nil {
		return err
	}
	cancel(nil)

	if utils.Options.IsInteractive {
		logrus.Info(locale.Loc("enter_to_exit", nil))
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
	}

	return nil
}

func (c *CLI) HandleMessage(msg *messages.Message) *messages.Message {
	switch data := msg.Data.(type) {
	case messages.RequestLogin:
		deviceType := &xbox.DeviceTypeAndroid
		if data.Wait {
			utils.Auth.Login(c.ctx, deviceType)
		} else {
			go utils.Auth.Login(c.ctx, deviceType)
		}
	}
	return nil
}
