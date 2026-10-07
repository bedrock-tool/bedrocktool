package utils

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
)

var panicStack string
var panicErr error
var panicMu sync.Mutex

// Fatal reports must not use stdout: the CLI tees it through an asynchronous
// pipe, and ErrorHandler exits without waiting for that pipe to drain. Stderr is
// redirected directly to the log file by setupLogging on supported platforms.
func PrintPanic(err error) {
	panicMu.Lock()
	defer panicMu.Unlock()
	stack := make([]byte, 32*1024)
	panicStack = string(stack[:runtime.Stack(stack, false)])
	panicErr = err
	reason := redactFatalReason(fmt.Sprint(err))
	if len(reason) > 4096 {
		reason = reason[:4096] + " [truncated]"
	}
	report := fmt.Sprintf("%s\n--COPY FROM HERE--\nFatal: exit_code=1\nVersion: %s\nError: %s\nstacktrace from panic: \n%s\n--END COPY HERE--\n\n%s\n%s\n",
		locale.Loc("fatal_error", nil), Version, reason, panicStack,
		locale.Loc("report_issue", nil), locale.Loc("used_extra_debug_report", nil))
	// One synchronous write bypasses the stdout drain race. Do not include argv:
	// it may contain credentials. Sync is best effort (e.g. terminals/pipes).
	_, _ = os.Stderr.WriteString(report)
	_ = os.Stderr.Sync()
}

var fatalSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[^\s,;]+`),
	regexp.MustCompile(`(?i)((?:access_token|refresh_token|id_token|authorization|password|client_secret|token)["']?\s*[:=]\s*["']?)[^\s,"';}]+`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`),
}

func redactFatalReason(reason string) string {
	for _, pattern := range fatalSecretPatterns {
		reason = pattern.ReplaceAllStringFunc(reason, func(match string) string {
			groups := pattern.FindStringSubmatch(match)
			if len(groups) > 1 {
				return groups[1] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return strings.ReplaceAll(reason, "\x00", "")
}
