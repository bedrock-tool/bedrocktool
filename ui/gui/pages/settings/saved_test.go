package settings

import (
	"os"
	"path/filepath"
	"testing"

	"gioui.org/widget"
	"github.com/bedrock-tool/bedrocktool/utils/savedsettings"
)

// TestMain keeps the tests away from the real saved settings.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bedrocktool-settings")
	if err != nil {
		panic(err)
	}
	SavedPath = func() string { return filepath.Join(dir, "saved-settings.json") }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func resetSaved(t *testing.T, content string) {
	t.Helper()
	os.Remove(SavedPath())
	if content != "" {
		if err := os.WriteFile(SavedPath(), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	AddressInput.SetText("")
}

// Settings started with come back on the next launch, address included.
func TestSettingsRemembered(t *testing.T) {
	resetSaved(t, "")
	p := New(nil).(*Page)
	p.selectMode("worlds")
	w := p.settings["worlds"]
	w.widgets["generator"].widget.(*choiceInput).SetValue("normal")
	w.widgets["save-entities"].widget.(*widget.Bool).Value = false
	w.widgets["chunk-radius"].widget.(interface{ SetText(string) }).SetText("32")
	AddressInput.SetText("play.example.com")
	if _, err := w.Apply(); err != nil {
		t.Fatal(err)
	}
	p.remember("worlds", w)

	AddressInput.SetText("")
	p2 := New(nil).(*Page)
	w2 := p2.settings["worlds"]
	if v := w2.Values(); v["generator"] != "normal" || v["save-entities"] != "false" || v["chunk-radius"] != "32" {
		t.Errorf("not restored: %v", v)
	}
	// an option never changed keeps its default
	if v := w2.Values(); v["save-inventories"] != "true" {
		t.Errorf("save-inventories = %q, want the default true", v["save-inventories"])
	}
	p2.selectMode("worlds")
	if got := AddressInput.Text(); got != "play.example.com" {
		t.Errorf("address = %q", got)
	}
	// skins was never started: it gets the latest server, and switching
	// back and forth replaces only an address that was filled in
	p2.selectMode("skins")
	if got := AddressInput.Text(); got != "play.example.com" {
		t.Errorf("skins address = %q", got)
	}
	AddressInput.SetText("typed.example.com")
	p2.selectMode("worlds")
	if got := AddressInput.Text(); got != "typed.example.com" {
		t.Errorf("a typed address was replaced with %q", got)
	}
}

func TestSettingsBrokenFile(t *testing.T) {
	resetSaved(t, `{"modes":{"worlds":`)
	p := New(nil).(*Page)
	if v := p.settings["worlds"].Values(); v["generator"] != "void" || v["save-entities"] != "true" {
		t.Errorf("a broken file should leave the defaults: %v", v)
	}
	p.selectMode("worlds")
	if got := AddressInput.Text(); got != "" {
		t.Errorf("address = %q, want none", got)
	}
}

// Values that don't fit any more (an option removed, a choice renamed) are
// skipped.
func TestSettingsStaleValues(t *testing.T) {
	s := &savedsettings.Saved{}
	s.SetMode("worlds", savedsettings.Mode{Options: map[string]string{"generator": "amplified", "gone": "true", "image": "maybe", "map-art": "true"}})
	if err := s.Save(SavedPath()); err != nil {
		t.Fatal(err)
	}
	AddressInput.SetText("")
	p := New(nil).(*Page)
	v := p.settings["worlds"].Values()
	if v["generator"] != "void" || v["image"] != "false" || v["map-art"] != "true" {
		t.Errorf("got %v", v)
	}
	if _, err := p.settings["worlds"].Apply(); err != nil {
		t.Errorf("apply: %v", err)
	}
}

// Replays are files, not servers: they aren't recent servers.
func TestReplayNotRecent(t *testing.T) {
	resetSaved(t, "")
	p := New(nil).(*Page)
	p.selectMode("worlds")
	AddressInput.SetText("C:/captures/session.pcap2")
	p.remember("worlds", p.settings["worlds"])
	if len(p.saved.Recent) != 0 {
		t.Errorf("recent = %q", p.saved.Recent)
	}
	AddressInput.SetText("realm:Magic Realm")
	p.remember("worlds", p.settings["worlds"])
	if len(p.saved.Recent) != 1 || p.saved.Recent[0] != "realm:Magic Realm" {
		t.Errorf("recent = %q", p.saved.Recent)
	}
}
