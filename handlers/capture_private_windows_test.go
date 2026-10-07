package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateCaptureWindowsDACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "captures")
	// Tighten a pre-existing directory as well as securely creating a new one.
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := prepareCaptureDirectory(dir); err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(t.TempDir(), "captures")
	if err := prepareCaptureDirectory(newDir); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "journal.pcap2")
	f, err := createPrivateCapture(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := "D:P(A;;FA;;;" + user.User.Sid.String() + ")"
	for _, path := range []string{dir, newDir, file} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil || acl.AceCount != 1 {
			t.Fatalf("%s: expected one user-only ACE, got %v %v", path, acl, err)
		}
		control, _, err := sd.Control()
		if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatalf("%s DACL is not protected: control=%v err=%v", path, control, err)
		}
		// Windows may retain the auto-inherited control bit after replacing
		// an existing DACL; it does not add ACEs or enable inheritance.
		if got := sd.String(); got != want && got != "D:PAI"+want[3:] {
			t.Fatalf("%s DACL = %s, want %s (optional AI control bit)", path, got, want)
		}
	}
}
