package main

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

func init() {
	stdout := windows.Handle(os.Stdout.Fd())
	var originalMode uint32

	windows.GetConsoleMode(stdout, &originalMode)
	windows.SetConsoleMode(stdout, originalMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
	// SetStdHandle does not affect prior references to stderr
	os.Stderr = f
}
