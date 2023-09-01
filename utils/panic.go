package utils

import (
	"os"
	"runtime/debug"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
)

var panicStack string
var panicErr error

func PrintPanic(err error) {
	panicErr = err
	logrus.Errorf(locale.Loc("fatal_error", nil))
	println("")
	println("--COPY FROM HERE--")
	logrus.Infof("Version: %s", Version)
	logrus.Infof("Cmdline: %s", os.Args)
	logrus.Errorf("Error: %s", err)
	panicStack = string(debug.Stack())
	println("stacktrace from panic: \n" + panicStack)
	println("--END COPY HERE--")
	println("")
	println(locale.Loc("report_issue", nil))
	if Options.ExtraDebug {
		println(locale.Loc("used_extra_debug_report", nil))
	}
}

func UploadPanic() {
	if panicErr == nil {
		return
	}
}

func UploadError() {

}
