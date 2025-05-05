package utils

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/bedrock-tool/bedrocktool/locale"
)

var panicStack string
var panicErr error

func PrintPanic(err error) {
	panicStack = string(debug.Stack())
	panicErr = err
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
