package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"
)

var panicStack string
var panicErr error

func PrintPanic(err error) {
	panicStack = string(debug.Stack())
	panicErr = err
	fmt.Printf(locale.Loc("fatal_error", nil))
	fmt.Println("")
	fmt.Println("--COPY FROM HERE--")
	fmt.Printf("Version: %s", updater.Version)
	fmt.Printf("Cmdline: %s", os.Args)
	fmt.Printf("Error: %s", err)
	fmt.Println("stacktrace from panic: \n" + panicStack)
	fmt.Println("--END COPY HERE--")
	fmt.Println("")
	fmt.Println(locale.Loc("report_issue", nil))
	if Options.ExtraDebug {
		fmt.Println(locale.Loc("used_extra_debug_report", nil))
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
		Version:     updater.Version,
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

	errorServer := updater.UpdateServer + "errors/"
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
