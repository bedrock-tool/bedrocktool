package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/bedrock-tool/bedrocktool/utils/xbox"
	"github.com/rifflock/lfshook"

	_ "github.com/bedrock-tool/bedrocktool/subcommands"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/merge"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/render"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/skins"
	_ "github.com/bedrock-tool/bedrocktool/subcommands/world"

	"github.com/sirupsen/logrus"
)

var uis = map[string]ui.UI{}

func selectUI() ui.UI {
	if flag.CommandLine.NArg() == 0 {
		utils.Options.IsInteractive = true
		if ui, ok := uis["gui"]; ok {
			return ui
		}
	}
	return uis["cli"]
}

type logFileWriter struct {
	w io.Writer
}

func (l logFileWriter) Write(b []byte) (int, error) {
	if utils.LogOff {
		return len(b), nil
	}
	return l.w.Write(b)
}

func setupLogging(isDebug bool) {
	logFile, err := os.Create("bedrocktool.log")
	if err != nil {
		logrus.Fatal(err)
	}

	rOut, wOut, err := os.Pipe()
	if err != nil {
		logrus.Fatal(err)
	}

	originalStdout := os.Stdout
	logWriter := logFileWriter{w: logFile}
	go func() {
		m := io.MultiWriter(originalStdout, logWriter)
		io.Copy(m, rOut)
	}()

	os.Stdout = wOut
	redirectStderr(wOut)

	logrus.SetLevel(logrus.DebugLevel)
	if isDebug {
		logrus.SetLevel(logrus.TraceLevel)
	}
	logrus.SetOutput(originalStdout)
	logrus.AddHook(lfshook.NewHook(logFile, &logrus.TextFormatter{
		DisableColors: true,
	}))
}

type logTransport struct {
	rt http.RoundTripper
}

func (t *logTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	logrus.Tracef("Request %s", req.URL.String())
	return t.rt.RoundTrip(req)
}

func main() {
	isDebug := updater.Version == ""
	setupLogging(isDebug)
	log := logrus.WithField("part", "main")
	ctx, cancel := context.WithCancelCause(context.Background())

	utils.ErrorHandler = func(err error) {
		if isDebug {
			panic(err)
		}
		utils.PrintPanic(err)
		utils.UploadPanic()
		cancel(err)
		if utils.Options.IsInteractive {
			input := bufio.NewScanner(os.Stdin)
			input.Scan()
		}
		os.Exit(1)
	}

	if isDebug {
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
		log.Info(locale.Loc("bedrocktool_version", locale.Strmap{"Version": updater.Version}))
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.StringVar(&utils.Options.Env, "env", "prod", "api env")
	flag.BoolVar(&utils.Options.Debug, "debug", false, locale.Loc("debug_mode", nil))
	flag.BoolVar(&utils.Options.ExtraDebug, "extra-debug", false, locale.Loc("extra_debug", nil))
	flag.BoolVar(&utils.Options.Capture, "capture", false, "Capture pcap2 file")
	var trace bool
	flag.BoolVar(&trace, "trace", false, "trace log")

	err := flag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		logrus.Fatal(err)
	}

	if trace {
		logrus.SetLevel(logrus.TraceLevel)
	}

	err = utils.Auth.Startup()
	if err != nil {
		logrus.Fatal(err)
	}

	ui := selectUI()

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel(errors.New("program closing"))
	}()

	if !ui.Init() {
		log.Error("Failed to init UI!")
		return
	}
	err = ui.Start(ctx, cancel)
	cancel(err)
	if err != nil {
		log.Error(err)
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
func (c *TransCMD) Execute(ctx context.Context) error {
	const (
		BlackFg = "\033[30m"
		Bold    = "\033[1m"
		Blue    = "\033[46m"
		Pink    = "\033[45m"
		White   = "\033[47m"
		Reset   = "\033[0m"
	)
	if c.auth {
		if utils.Auth.LoggedIn() {
			logrus.Info("Already Logged in")
		} else {
			utils.Auth.Login(ctx, &xbox.DeviceTypeAndroid)
		}
	}
	fmt.Println(BlackFg + Bold + Blue + " Trans " + Pink + " Rights " + White + " Are " + Pink + " Human " + Blue + " Rights " + Reset)
	return nil
}

func init() {
	commands.RegisterCommand(&TransCMD{})
}
