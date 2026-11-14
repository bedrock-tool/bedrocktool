package locale

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadFile(t *testing.T, name string) map[string]map[string]string {
	t.Helper()
	data, err := localesFS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]map[string]string
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %s", name, err)
	}
	return m
}

var templateVar = regexp.MustCompile(`{{\s*\.(\w+)\s*}}`)

func vars(s string) []string {
	var out []string
	for _, m := range templateVar.FindAllStringSubmatch(s, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	slices.Sort(out)
	return out
}

// Every language has the same keys, plural forms and template fields as English.
func TestTranslationsMatchEnglish(t *testing.T) {
	en := loadFile(t, "en.yaml")
	entries, _ := localesFS.ReadDir(".")
	for _, e := range entries {
		if e.Name() == "en.yaml" || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		other := loadFile(t, e.Name())
		for key, forms := range en {
			tr, ok := other[key]
			if !ok {
				t.Errorf("%s: missing %s", e.Name(), key)
				continue
			}
			for form, text := range forms {
				if _, ok := tr[form]; !ok {
					t.Errorf("%s: %s has no %q form", e.Name(), key, form)
				}
				if got, want := vars(tr[form]), vars(text); !slices.Equal(got, want) {
					t.Errorf("%s: %s.%s uses %v, English uses %v", e.Name(), key, form, got, want)
				}
			}
		}
		for key := range other {
			if _, ok := en[key]; !ok {
				t.Errorf("%s: %s is not in en.yaml", e.Name(), key)
			}
		}
	}
}

var keyRefs = []*regexp.Regexp{
	regexp.MustCompile(`Locm?\("([a-z0-9_]+)"`),
	regexp.MustCompile(`"locale\.([a-z0-9_]+)"`),
	regexp.MustCompile(`(?:opt|desc):"locale\.([a-z0-9_]+)"`),
	regexp.MustCompile(`"(gui_save_step_[a-z0-9_]+)"`), // labels kept in a table
}

// choicesTag finds the choices of a setting's struct tag (one with a flag,
// so an example in a comment doesn't count).
var choicesTag = regexp.MustCompile("flag:\"[^\"]+\"[^`\\n]*choices:\"([a-z0-9_,]+)\"")

// Every key the code asks for exists, and every key in en.yaml is used.
func TestKeysUsed(t *testing.T) {
	en := loadFile(t, "en.yaml")
	used := map[string]string{}
	root := ".."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// .claude holds other checkouts (session worktrees) with their
			// own, older copies of the code
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "gophertunnel", "dragonfly", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // unreadable (encrypted) files have no keys we can check
		}
		for _, re := range keyRefs {
			for _, m := range re.FindAllSubmatch(data, -1) {
				used[string(m[1])] = path
			}
		}
		// a setting's choices are shown as choice_<value>
		for _, m := range choicesTag.FindAllSubmatch(data, -1) {
			for _, c := range strings.Split(string(m[1]), ",") {
				used["choice_"+c] = path
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, path := range used {
		if _, ok := en[key]; !ok {
			t.Errorf("%s uses %s, which en.yaml doesn't have", path, key)
		}
	}
	for key := range en {
		if _, ok := used[key]; !ok {
			t.Errorf("en.yaml has %s, which nothing uses", key)
		}
	}
}

func TestFallback(t *testing.T) {
	t.Cleanup(func() { SetLanguage("en") })

	SetLanguage("de")
	if got := Loc("gui_close", nil); got != "Schließen" {
		t.Errorf("de gui_close = %q", got)
	}
	if got := Locm("saving_world", Strmap{"Name": "w"}, 1); got != "Speichere Welt w mit 1 Chunk" {
		t.Errorf("de saving_world = %q", got)
	}

	for _, name := range []string{"", "xx", "not a language", "de_AT", "en-US", "fr"} {
		SetLanguage(name)
		got := Loc("gui_close", nil)
		if got != "Close" && got != "Schließen" {
			t.Errorf("%q: gui_close = %q", name, got)
		}
	}
	SetLanguage("fr")
	if got := Loc("gui_close", nil); got != "Close" {
		t.Errorf("fr should fall back to English, got %q", got)
	}
	if got := Loc("no_such_key", nil); got != "no_such_key" {
		t.Errorf("missing key = %q", got)
	}
	if got := Locm("no_such_key", nil, 3); got != "no_such_key" {
		t.Errorf("missing plural key = %q", got)
	}
	// missing template data must not crash
	_ = Loc("saved", nil)
	_ = Locm("saving_world", nil, 2)
}
