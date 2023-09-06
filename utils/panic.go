package utils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
)

var panicStack string
var panicErr error

func PrintPanic(err error) {
	panicStack = string(debug.Stack())
	panicErr = err
	logrus.Errorf(locale.Loc("fatal_error", nil))
	println("")
	println("--COPY FROM HERE--")
	logrus.Infof("Version: %s", Version)
	logrus.Infof("Cmdline: %s", os.Args)
	logrus.Errorf("Error: %s", err)
	println("stacktrace from panic: \n" + panicStack)
	println("--END COPY HERE--")
	println("")
	println(locale.Loc("report_issue", nil))
	if Options.ExtraDebug {
		println(locale.Loc("used_extra_debug_report", nil))
	}
}

type errorReport struct {
	Version     string
	OS          string
	ErrorString string
	Error       any
	Stacktrace  string
}

func UploadPanic() {
	if panicErr == nil {
		return
	}
	UploadError(panicErr)
}

func UploadError(err error) {
	report := errorReport{
		Version:     Version,
		OS:          runtime.GOOS,
		ErrorString: err.Error(),
		Error:       err,
		Stacktrace:  panicStack,
	}

	body := bytes.NewBuffer(nil)
	err = json.NewEncoder(body).Encode(report)
	if err != nil {
		logrus.Error(err)
		return
	}

	errorServer := updateServer + "errors/"
	req, err := http.NewRequest("PUT", errorServer+"/submit", body)
	if err != nil {
		logrus.Error(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Error(err)
		return
	}

	if res.StatusCode != 200 {
		logrus.Errorf("Upload Error Status: %d", res.StatusCode)
	}
}
