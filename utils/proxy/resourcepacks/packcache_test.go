package resourcepacks

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestParsePackID(t *testing.T) {
	want := uuid.MustParse("5f6dc6b5-0c8c-4f5c-8e5f-3c3a4d2b1a00")
	for _, s := range []string{
		want.String(),
		want.String() + "_1.0.0", // 42 characters, #355
		want.String() + "_10.2.3",
	} {
		got, err := parsePackID(s)
		if err != nil {
			t.Fatalf("parsePackID(%q): %v", s, err)
		}
		if got != want {
			t.Fatalf("parsePackID(%q) = %v, want %v", s, got, want)
		}
	}
	if _, err := parsePackID("not-a-uuid_1.0.0"); err == nil {
		t.Fatal("parsePackID accepted an invalid id")
	}
}

// A cached pack with no manifest (#297) is removed so it gets downloaded
// again, instead of failing every join until packcache is deleted by hand.
func TestPackCacheBrokenPackIsRemoved(t *testing.T) {
	t.Chdir(t.TempDir())
	c := &packCache{}
	id := uuid.New()

	p := c.cachedPath(id, "1.0.0")
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("textures/readme.txt")
	_, _ = w.Write([]byte("no manifest here"))
	_ = zw.Close()
	_ = f.Close()

	if !c.Has(id, "1.0.0") {
		t.Fatal("Has = false for a file that exists")
	}
	_, err = c.Get(id, "1.0.0")
	if !errors.Is(err, errBadCachedPack) {
		t.Fatalf("Get err = %v, want errBadCachedPack", err)
	}
	if c.Has(id, "1.0.0") {
		t.Fatal("broken pack is still in the cache")
	}
}
