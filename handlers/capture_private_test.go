package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateCaptureDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.pcap2")
	f, err := createPrivateCapture(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("retained packet"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := createPrivateCapture(path)
	if err == nil {
		second.Close()
		t.Fatal("existing journal overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "retained packet" {
		t.Fatalf("existing journal changed: %q %v", data, err)
	}
}

func TestCaptureDirectoryRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "captures")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := prepareCaptureDirectory(link); err == nil {
		t.Fatal("followed capture directory symlink")
	}
}
