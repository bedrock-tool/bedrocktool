package savedsettings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "saved-settings.json")

	s, err := Load(path)
	if err != nil || len(s.Modes) != 0 || len(s.Recent) != 0 {
		t.Fatalf("missing file: got %+v, %v; want empty and no error", s, err)
	}

	s.SetMode("worlds", Mode{Address: " play.example.com ", Options: map[string]string{"generator": "normal", "save-entities": "false"}})
	s.SetMode("skins", Mode{Address: "realm:My Realm"})
	s.AddRecent("play.example.com")
	s.AddRecent("realm:My Realm")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := got.Mode("worlds")
	if !ok || w.Address != "play.example.com" || w.Options["generator"] != "normal" || w.Options["save-entities"] != "false" {
		t.Errorf("worlds = %+v, %v", w, ok)
	}
	if sk, _ := got.Mode("skins"); sk.Address != "realm:My Realm" {
		t.Errorf("skins = %+v", sk)
	}
	if _, ok := got.Mode("packs"); ok {
		t.Error("packs was never started")
	}
	if want := []string{"realm:My Realm", "play.example.com"}; !slices.Equal(got.Recent, want) {
		t.Errorf("recent = %q, want %q", got.Recent, want)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary file was left behind")
	}
}

func TestLoadBroken(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"truncated": `{"modes":{"worlds":{"address":"a`,
		"garbage":   "\x00\x01not json",
		"wrongtype": `{"modes":[1,2,3],"recent_servers":"x"}`,
		"empty":     "",
	} {
		path := filepath.Join(dir, name+".json")
		os.WriteFile(path, []byte(content), 0o644)
		s, err := Load(path)
		if err == nil {
			t.Errorf("%s: want an error to log", name)
		}
		if s == nil || len(s.Modes) != 0 || len(s.Recent) != 0 {
			t.Errorf("%s: want defaults, got %+v", name, s)
		}
		// and it can still be saved over
		s.AddRecent("x.example.com")
		if err := s.Save(path); err != nil {
			t.Errorf("%s: saving over it: %v", name, err)
		}
		if s2, err := Load(path); err != nil || len(s2.Recent) != 1 {
			t.Errorf("%s: after saving over it: %+v, %v", name, s2, err)
		}
	}
}

func TestRecent(t *testing.T) {
	s := &Saved{}
	s.AddRecent("a.example.com")
	s.AddRecent("realm:Magic")
	s.AddRecent("")
	s.AddRecent("   ")
	s.AddRecent("A.example.com") // same server: moves to the front, once
	if want := []string{"A.example.com", "realm:Magic"}; !slices.Equal(s.Recent, want) {
		t.Fatalf("dedupe: %q, want %q", s.Recent, want)
	}

	for i := range MaxRecent + 5 {
		s.AddRecent(fmt.Sprintf("server%d.example.com", i))
	}
	if len(s.Recent) != MaxRecent {
		t.Fatalf("cap: %d kept, want %d", len(s.Recent), MaxRecent)
	}
	if s.Recent[0] != fmt.Sprintf("server%d.example.com", MaxRecent+4) {
		t.Errorf("newest isn't first: %q", s.Recent)
	}
	if slices.Contains(s.Recent, "realm:Magic") {
		t.Error("the oldest should have been dropped")
	}

	s.RemoveRecent(" SERVER14.example.com ")
	s.RemoveRecent("not-there.example.com")
	if len(s.Recent) != MaxRecent-1 || slices.Contains(s.Recent, "server14.example.com") {
		t.Errorf("remove: %q", s.Recent)
	}
}

func TestLoadCleansRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	list := `"a", "A", " ", "b"`
	for i := range 20 {
		list += fmt.Sprintf(`, "s%d"`, i)
	}
	os.WriteFile(path, []byte(`{"recent_servers":[`+list+`]}`), 0o644)
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Recent) != MaxRecent || s.Recent[0] != "a" || s.Recent[1] != "b" {
		t.Errorf("recent = %q", s.Recent)
	}
}
