package android

import (
	"bytes"
	"os"
	"runtime/debug"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
)

// When Go crashes (a panic nothing recovered, in any goroutine) it prints
// the error to stderr and exits. On Android stderr only reaches logcat,
// which users can't read, so the runtime also writes it to crashFile.
// The next start finds it there and offers to report it.
const (
	crashFile     = "crash-pending.txt"    // written by the runtime as this run crashes
	lastCrashFile = "last-crash.txt"       // a previous run's crash, until dismissed
	logFile       = "bedrocktool.log"      // what cmd/bedrocktool logs to
	prevLogFile   = "bedrocktool.prev.log" // the log of the run that crashed
)

var (
	crashMu   sync.Mutex
	lastCrash string
)

// Setup runs first thing at startup, in the data folder, before logging
// starts (which empties bedrocktool.log).
func Setup() {
	setupUpdates()
	if b, err := os.ReadFile(utils.PathData(crashFile)); err == nil && len(bytes.TrimSpace(b)) > 0 {
		_ = os.WriteFile(utils.PathData(lastCrashFile), b, 0o644)
		// keep the log of the run that crashed; this run starts a new one
		_ = os.Rename(utils.PathData(logFile), utils.PathData(prevLogFile))
	}
	if b, err := os.ReadFile(utils.PathData(lastCrashFile)); err == nil {
		lastCrash = string(b)
	}

	// removed first: a file the app can't write (one pushed with adb, say)
	// would make Create fail and leave crashes uncaught
	_ = os.Remove(utils.PathData(crashFile))
	f, err := os.Create(utils.PathData(crashFile))
	if err != nil {
		return
	}
	// the runtime keeps its own copy of the file descriptor
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	f.Close()
}

// LastCrash is the crash output of a previous run, or "".
func LastCrash() string {
	crashMu.Lock()
	defer crashMu.Unlock()
	return lastCrash
}

// DismissCrash forgets the previous run's crash, so it isn't offered again.
func DismissCrash() {
	crashMu.Lock()
	defer crashMu.Unlock()
	lastCrash = ""
	_ = os.Remove(utils.PathData(lastCrashFile))
}
