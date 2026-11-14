package utils

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
)

var panicStack string
var panicErr error

func PrintPanic(err error) {
	panicStack = string(debug.Stack())
	panicErr = err
	// the GUI has no console, so the report also goes to the log and a file
	logrus.Errorf("%s\n%s", locale.Loc("panic_fatal_error", locale.Strmap{"Err": err}), panicStack)
	crashFile := fmt.Sprintf("crash-%s.txt", time.Now().Format("2006-01-02_15-04-05.000"))
	report := fmt.Sprintf("Version: %s\nCmdline: %s\nError: %s\n\n%s", Version, os.Args, err, panicStack)
	if werr := os.WriteFile(PathData(crashFile), []byte(report), 0o644); werr == nil {
		logrus.Error(locale.Loc("crash_report_written", locale.Strmap{"Path": crashFile}))
	}
	fmt.Println(locale.Loc("fatal_error", nil))
	fmt.Println("--COPY FROM HERE--")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Cmdline: %s\n", os.Args)
	fmt.Printf("Error: %s\n", err)
	fmt.Println("stacktrace from panic: \n" + panicStack)
	fmt.Println("--END COPY HERE--")
	fmt.Println("")
	fmt.Println(locale.Loc("report_issue", nil))
	fmt.Println(locale.Loc("used_extra_debug_report", nil))
}
