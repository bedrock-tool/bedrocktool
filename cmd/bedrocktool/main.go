package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/cli"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/osabs"

	_ "github.com/bedrock-tool/bedrocktool/subcommands"

	"github.com/sirupsen/logrus"
)

var uis = map[string]ui.UI{}

func selectUI() ui.UI {
	if len(os.Args) == 1 {
		if ui, ok := uis["gui"]; ok {
			return ui
		} else {
			c := uis["cli"].(*cli.CLI)
			c.IsInteractive = true
		}
	}
	return uis["cli"]
}

func main() {
	if err := osabs.Init(); err != nil {
		panic(err)
	}
	baseDir := osabs.GetDataDir()
	if baseDir != "" {
		if err := os.Chdir(baseDir); err != nil {
			panic(err)
		}
	}

	setupLogging(utils.IsDebug())
	log := logrus.WithField("part", "main")
	ctx, cancel := context.WithCancelCause(context.Background())

	utils.ErrorHandler = func(err error) {
		if utils.IsDebug() {
			panic(err)
		}
		utils.PrintPanic(err)
		cancel(err)

		var IsInteractive bool
		if c, ok := uis["cli"].(*cli.CLI); ok {
			IsInteractive = c.IsInteractive
		}
		if IsInteractive {
			input := bufio.NewScanner(os.Stdin)
			input.Scan()
		}
		os.Exit(1)
	}

	if utils.IsDebug() {
		f, err := os.Create("cpu.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		defer func() {
			f, err := os.Create("mem.pprof")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			pprof.WriteHeapProfile(f)
		}()

		http.DefaultTransport = &logTransport{rt: http.DefaultTransport}
	} else {
		log.Info(locale.Loc("bedrocktool_version", locale.Strmap{"Version": utils.Version}))
	}

	env, ok := os.LookupEnv("BEDROCK_ENV")
	if !ok {
		env = "prod"
	}
	auth.Auth.SetEnv(env)
	tokenName, _ := os.LookupEnv("TOKEN_NAME")
	if err := auth.Auth.LoadAccount(tokenName); err != nil {
		logrus.Fatal(err)
	}

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel(errors.New("program closing"))
	}()

	ui := selectUI()
	if err := ui.Init(); err != nil {
		log.Errorf("Failed to init UI %s", err)
		return
	}
	err := ui.Start(ctx, cancel)
	if err != nil {
		log.Error(err)
	}
}

type TransSettings struct {
	Auth bool `opt:"Auth" flag:"auth" desc:"locale.should_login_xbox"`
}

type TransCMD struct{}

func (TransCMD) Name() string {
	return "trans"
}

func (TransCMD) Description() string {
	return ""
}

func (TransCMD) Settings() any {
	return new(TransSettings)
}

func (TransCMD) Run(ctx context.Context, settings any) error {
	transSettings := settings.(*TransSettings)
	const (
		BlackFg = "\033[30m"
		Bold    = "\033[1m"
		Blue    = "\033[46m"
		Pink    = "\033[45m"
		White   = "\033[47m"
		Reset   = "\033[0m"
	)
	if transSettings.Auth {
		if auth.Auth.LoggedIn() {
			logrus.Info("Already Logged in")
		} else {
			auth.Auth.Login(ctx, &xbox.DeviceTypeAndroid, "")
		}
	}
	fmt.Println(BlackFg + Bold + Blue + " Trans " + Pink + " Rights " + White + " Are " + Pink + " Human " + Blue + " Rights " + Reset)
	return nil
}

func init() {
	commands.RegisterCommand(&TransCMD{})
}
