package android

// No build constraint: this runs on the PC too, reading the files as text.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var keyRef = regexp.MustCompile(`\bT\("([a-z0-9_]+)"`)

// Every key the Android code asks for is in locale/en.yaml, and every key
// there is used. The shared locale test only knows the shared files.
func TestAndroidKeysUsed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("locale", "en.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]map[string]string
	if err := yaml.Unmarshal(data, &en); err != nil {
		t.Fatal(err)
	}

	// T("...") calls in every *_android.go file of the repository
	used := map[string]string{}
	err = filepath.WalkDir(filepath.Join("..", "..", ".."), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != "." && name != "..") || name == "gophertunnel" || name == "dragonfly" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_android.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range keyRef.FindAllSubmatch(src, -1) {
			used[string(m[1])] = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(used) == 0 {
		t.Fatal("found no T(\"...\") calls in *_android.go files")
	}
	for key, path := range used {
		if _, ok := en[key]; !ok {
			t.Errorf("%s uses %s, which locale/en.yaml doesn't have", path, key)
		}
	}
	for key := range en {
		if _, ok := used[key]; !ok {
			t.Errorf("locale/en.yaml has %s, which nothing uses", key)
		}
	}
}
