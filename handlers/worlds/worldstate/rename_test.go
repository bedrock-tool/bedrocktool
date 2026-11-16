package worldstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// openTestWorld is a world with its database open in a temporary folder,
// as one is once its first chunks are stored.
func openTestWorld(t *testing.T) *World {
	t.Helper()
	w := newTestWorld(t)
	w.Name = "world-2"
	w.Folder = filepath.Join(t.TempDir(), "world-2")
	if err := os.MkdirAll(w.Folder, 0o777); err != nil {
		t.Fatal(err)
	}
	provider, err := w.openProvider(w.Folder)
	if err != nil {
		t.Fatal(err)
	}
	w.provider = provider
	t.Cleanup(func() { w.provider.Close() })
	return w
}

// writable says whether the world's database still takes writes.
func writable(t *testing.T, w *World) {
	t.Helper()
	if err := w.provider.LDB().Put([]byte("test"), []byte("ok"), nil); err != nil {
		t.Fatalf("database doesn't take writes after the rename: %v", err)
	}
}

func TestRenameMovesFolder(t *testing.T) {
	w := openTestWorld(t)
	old := w.Folder
	folder := filepath.Join(filepath.Dir(old), "Murder Hub")

	if err := w.Rename("Murder Hub 11/16/2026", folder); err != nil {
		t.Fatal(err)
	}
	if w.Folder != folder || w.Name != "Murder Hub 11/16/2026" {
		t.Errorf("Folder %q, Name %q", w.Folder, w.Name)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old folder still there: %v", err)
	}
	writable(t, w)
}

// A folder that can't be moved used to leave the world with its database
// closed, so every chunk stored after failed with "leveldb: closed" and the
// world was never saved.
func TestRenameFailureKeepsDatabaseOpen(t *testing.T) {
	w := openTestWorld(t)
	old := w.Folder

	// a file where the new folder's parent should be: the move fails on
	// every system
	blocker := filepath.Join(filepath.Dir(old), "blocker")
	if err := os.WriteFile(blocker, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	err := w.Rename("Murder Hub", filepath.Join(blocker, "Murder Hub"))
	if err == nil {
		t.Fatal("no error for a folder that couldn't be moved")
	}
	if w.Folder != old {
		t.Errorf("Folder = %q, want the old %q", w.Folder, old)
	}
	if w.Name != "Murder Hub" {
		t.Errorf("Name = %q, want the new name", w.Name)
	}
	writable(t, w)
}

// The folder isn't moved while its resource packs are still being copied
// into it, and the state lock stays free while Rename waits, so chunks can
// still be stored.
func TestRenameWaitsForResourcePacks(t *testing.T) {
	w := openTestWorld(t)
	copied := make(chan struct{})
	w.resourcePacksCopied = copied
	folder := filepath.Join(filepath.Dir(w.Folder), "renamed")

	done := make(chan error, 1)
	go func() { done <- w.Rename("renamed", folder) }()

	select {
	case err := <-done:
		t.Fatalf("Rename returned while packs were copied: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if !w.stateLock.TryLock() {
		t.Fatal("state lock held while waiting for the packs")
	}
	w.stateLock.Unlock()

	close(copied)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Rename didn't finish after the packs were copied")
	}
	if w.Folder != folder {
		t.Errorf("Folder = %q, want %q", w.Folder, folder)
	}
	writable(t, w)
}

// As in the freeze it fixes: a pack file is still open in the folder when
// the rename comes (Windows refuses to move the folder then), and the copy
// finishes a moment later.
func TestRenameWhilePackFileOpen(t *testing.T) {
	w := openTestWorld(t)
	packDir := filepath.Join(w.Folder, "resource_packs", "HiveDR")
	if err := os.MkdirAll(packDir, 0o777); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(packDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	copied := make(chan struct{})
	w.resourcePacksCopied = copied
	go func() {
		time.Sleep(200 * time.Millisecond)
		f.Close()
		close(copied)
	}()

	folder := filepath.Join(filepath.Dir(w.Folder), "Murder Hub 11_16_2026")
	if err := w.Rename("Murder Hub 11/16/2026", folder); err != nil {
		t.Fatal(err)
	}
	if w.Folder != folder {
		t.Errorf("Folder = %q, want %q", w.Folder, folder)
	}
	if _, err := os.Stat(filepath.Join(folder, "resource_packs", "HiveDR", "manifest.json")); err != nil {
		t.Errorf("pack not moved with the folder: %v", err)
	}
	writable(t, w)
}

func TestRenameSavedWorld(t *testing.T) {
	w := openTestWorld(t)
	w.cancelCtx(errFinished)
	if err := w.Rename("late", filepath.Join(filepath.Dir(w.Folder), "late")); err == nil {
		t.Error("renamed a world that was already saved")
	}
}
