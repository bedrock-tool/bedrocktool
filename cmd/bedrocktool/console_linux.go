//go:build linux && !android

package main

import (
	"log"
	"os"
	"syscall"
)

func redirectStderr(f *os.File) {
	err := syscall.Dup3(int(f.Fd()), int(os.Stderr.Fd()), 0)
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
}
