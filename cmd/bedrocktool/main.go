package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	_ "github.com/bedrock-tool/bedrocktool/subcommands"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/skins"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/world"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Errorf(locale.Loc("fatal_error", nil))
			println("")
			println("--COPY FROM HERE--")
			logrus.Infof("Version: %s", utils.Version)
			logrus.Infof("Cmdline: %s", os.Args)
			logrus.Errorf("Error: %s", err)
			println("stacktrace from panic: \n" + string(debug.Stack()))
			println("--END COPY HERE--")
			println("")
			println(locale.Loc("report_issue", nil))
			if utils.G_interactive {
				input := bufio.NewScanner(os.Stdin)
				input.Scan()
			}
			os.Exit(1)
		}
	}()

	logrus.SetLevel(logrus.DebugLevel)
	if utils.Version != "" {
		logrus.Infof(locale.Loc("bedrocktool_version", locale.Strmap{"Version": utils.Version}))
	}

	newVersion, err := utils.Updater.UpdateAvailable()
	if err != nil {
		logrus.Error(err)
	}

	if newVersion != "" && utils.Version != "" {
		logrus.Infof(locale.Loc("update_available", locale.Strmap{"Version": newVersion}))
	}

	ctx, cancel := context.WithCancel(context.Background())

	flag.BoolVar(&utils.G_debug, "debug", false, locale.Loc("debug_mode", nil))
	flag.BoolVar(&utils.G_preload_packs, "preload", false, locale.Loc("preload_packs", nil))
	enable_dns := flag.Bool("dns", false, locale.Loc("enable_dns", nil))

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.ImportantFlag("debug")
	subcommands.ImportantFlag("dns")
	subcommands.ImportantFlag("preload")
	subcommands.HelpCommand()

	{ // interactive input
		if len(os.Args) < 2 {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Println(locale.Loc("available_commands", nil))
				for name, desc := range utils.ValidCMDs {
					fmt.Printf("\t%s\t%s\n", name, desc)
				}
				fmt.Println(locale.Loc("use_to_run_command", nil))

				cmd, cancelled := utils.User_input(ctx, locale.Loc("input_command", nil))
				if cancelled {
					return
				}
				_cmd := strings.Split(cmd, " ")
				os.Args = append(os.Args, _cmd...)
				utils.G_interactive = true
			}
		}
	}

	flag.Parse()

	if *enable_dns {
		utils.InitDNS()
	}

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		println("cancelling")
		cancel()
	}()

	subcommands.Execute(ctx)

	if utils.G_interactive {
		logrus.Info(locale.Loc("enter_to_exit", nil))
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
	}
}

type TransCMD struct {
	auth bool
}

func (*TransCMD) Name() string     { return "trans" }
func (*TransCMD) Synopsis() string { return "" }

func (c *TransCMD) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.auth, "auth", false, locale.Loc("should_login_xbox", nil))
}

func (c *TransCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *TransCMD) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	const (
		BLACK_FG = "\033[30m"
		BOLD     = "\033[1m"
		BLUE     = "\033[46m"
		PINK     = "\033[45m"
		WHITE    = "\033[47m"
		RESET    = "\033[0m"
	)
	if c.auth {
		utils.GetTokenSource()
	}
	fmt.Println(BLACK_FG + BOLD + BLUE + " Trans " + PINK + " Rights " + WHITE + " Are " + PINK + " Human " + BLUE + " Rights " + RESET)
	return 0
}

func init() {
	utils.RegisterCommand(&TransCMD{})
}
