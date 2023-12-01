package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/gregwebs/go-recovery"
	"github.com/rifflock/lfshook"

	_ "github.com/bedrock-tool/bedrocktool/subcommands"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/skins"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/world"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

var uis = map[string]ui.UI{}

func selectUI() ui.UI {
	var ui ui.UI
	if len(os.Args) < 2 {
		var ok bool
		ui, ok = uis["gui"]
		if !ok {
			ui = uis["tui"]
		}
		utils.Options.IsInteractive = true
	} else {
		ui = uis["cli"]
	}
	return ui
}

func updateCheck(ui ui.UI) {
	update, err := updater.UpdateAvailable()
	if err != nil {
		logrus.Warn(err)
		return
	}
	isNew := update.Version != updater.Version

	if isNew {
		logrus.Infof(locale.Loc("update_available", locale.Strmap{"Version": update.Version}))
		ui.Message(messages.UpdateAvailable{Version: update.Version})
	}
}

func main() {
	{
		f, err := os.Create("bedrocktool.log")
		if err != nil {
			logrus.Warn(err)
		} else {
			logrus.AddHook(lfshook.NewHook(f, &logrus.TextFormatter{
				DisableColors: true,
			}))
		}
	}

	isDebug := updater.Version == ""

	if isDebug {
		f, err := os.Create("cpu.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	recovery.ErrorHandler = func(err error) {
		utils.PrintPanic(err)
		utils.UploadPanic()
	}

	defer func() {
		// dont catch panic if not release verion
		if updater.Version == "" {
			logrus.Warn("no version set")
			return
		}
		if err := recover(); err != nil {
			if s, ok := err.(string); ok {
				err = errors.New(s)
			}
			recovery.ErrorHandler(err.(error))
			if utils.Options.IsInteractive {
				input := bufio.NewScanner(os.Stdin)
				input.Scan()
			}
			os.Exit(1)
		}
	}()

	logrus.SetLevel(logrus.DebugLevel)
	if !isDebug {
		logrus.Infof(locale.Loc("bedrocktool_version", locale.Strmap{"Version": updater.Version}))
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	utils.Auth.Ctx = ctx

	flag.StringVar(&utils.RealmsEnv, "realms-env", "", "realms env")
	flag.BoolVar(&utils.Options.Debug, "debug", false, locale.Loc("debug_mode", nil))
	flag.BoolVar(&utils.Options.ExtraDebug, "extra-debug", false, locale.Loc("extra_debug", nil))
	flag.StringVar(&utils.Options.PathCustomUserData, "userdata", "", locale.Loc("custom_user_data", nil))
	flag.String("lang", "", "lang")
	flag.BoolVar(&utils.Options.Capture, "capture", false, "Capture pcap2 file")

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.ImportantFlag("debug")
	subcommands.ImportantFlag("capture")
	subcommands.HelpCommand()

	ui := selectUI()

	if !isDebug {
		go updateCheck(ui)
	}

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel(errors.New("program closing"))
	}()

	if !ui.Init() {
		logrus.Error("Failed to init UI!")
		return
	}
	err := ui.Start(ctx, cancel)
	cancel(err)
	if err != nil {
		logrus.Error(err)
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
func (c *TransCMD) Execute(_ context.Context, ui ui.UI) error {
	const (
		BlackFg = "\033[30m"
		Bold    = "\033[1m"
		Blue    = "\033[46m"
		Pink    = "\033[45m"
		White   = "\033[47m"
		Reset   = "\033[0m"
	)
	if c.auth {
		_, err := utils.Auth.GetTokenSource()
		if err != nil {
			logrus.Error(err)
			return nil
		}
	}
	fmt.Println(BlackFg + Bold + Blue + " Trans " + Pink + " Rights " + White + " Are " + Pink + " Human " + Blue + " Rights " + Reset)
	return nil
}

/*
type CreateCustomDataCMD struct {
	path string
}

func (*CreateCustomDataCMD) Name() string     { return "create-customdata" }
func (*CreateCustomDataCMD) Synopsis() string { return "" }

func (c *CreateCustomDataCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.path, "path", "customdata.json", "where to save")
}

func (c *CreateCustomDataCMD) Execute(_ context.Context, ui ui.UI) error {
	var data proxy.CustomClientData
	fio, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer fio.Close()
	e := json.NewEncoder(fio)
	if err = e.Encode(data); err != nil {
		return err
	}
	return nil
}
*/

func init() {
	commands.RegisterCommand(&TransCMD{})
	//commands.RegisterCommand(&CreateCustomDataCMD{})
}
