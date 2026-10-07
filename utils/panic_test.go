package utils

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPrintPanicSurvivesImmediateExit(t *testing.T) {
	if os.Getenv("BEDROCKTOOL_FATAL_TEST") == "1" {
		_, w, err := os.Pipe()
		if err != nil {
			panic(err)
		}
		os.Stdout = w // No reader: reproduces an undrained logging pipe.
		os.Args = []string{"bedrocktool", "--token", "do-not-log-this-secret"}
		PrintPanic(errors.New("custom block initialization failed: access_token=secret-access-token Bearer secret-bearer-token " + strings.Repeat("x", 100000)))
		os.Exit(1)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPrintPanicSurvivesImmediateExit$")
	cmd.Env = append(os.Environ(), "BEDROCKTOOL_FATAL_TEST=1")
	log, err := os.CreateTemp(t.TempDir(), "fatal-*.log")
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = log // Match the recorder's direct, file-backed stderr.
	err = cmd.Run()
	_ = log.Close()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit status 1, got %v", err)
	}
	output, err := os.ReadFile(log.Name())
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{"exit_code=1", "custom block initialization failed", "Version:", "stacktrace", "--END COPY HERE--"} {
		if !strings.Contains(text, want) {
			t.Errorf("fatal output missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "do-not-log-this-secret") || strings.Contains(text, "secret-access-token") || strings.Contains(text, "secret-bearer-token") {
		t.Fatal("fatal output leaked credentials")
	}
	if len(text) > 40*1024 || !strings.Contains(text, "[truncated]") {
		t.Fatalf("fatal report not bounded: %d bytes", len(text))
	}
}
