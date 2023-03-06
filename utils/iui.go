package utils

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/subcommands"
)

type UI interface {
	Init()
	SetOptions(context.Context) bool
	Execute(context.Context) error
}

type InteractiveCLI struct {
	UI
}

func (c *InteractiveCLI) Init() {
}

func (c *InteractiveCLI) SetOptions(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		fmt.Println(locale.Loc("available_commands", nil))
		for name, cmd := range ValidCMDs {
			fmt.Printf("\t%s\t%s\n", name, cmd.Synopsis())
		}
		fmt.Println(locale.Loc("use_to_run_command", nil))

		cmd, cancelled := UserInput(ctx, locale.Loc("input_command", nil))
		if cancelled {
			return true
		}
		_cmd := strings.Split(cmd, " ")
		os.Args = append(os.Args, _cmd...)
	}

	flag.Parse()
	return false
}

func (c *InteractiveCLI) Execute(ctx context.Context) error {
	subcommands.Execute(ctx)
	return nil
}

var MakeGui = func() UI {
	return &InteractiveCLI{}
}
