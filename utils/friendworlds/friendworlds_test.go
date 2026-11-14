package friendworlds

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/go-xsapi/v2/mpsd"
	"github.com/google/uuid"
)

const supported = 2225

// activity is an activity handle as the session directory returns it for a
// friend's world; change edits its custom properties first.
func activity(host, name string, change func(m map[string]any)) mpsd.ActivityHandle {
	props := map[string]any{
		"hostName":         host,
		"ownerId":          host + "-xuid",
		"worldName":        name,
		"version":          "1.26.60",
		"protocol":         supported,
		"MemberCount":      2,
		"MaxMemberCount":   8,
		"Joinability":      "joinable_by_friends",
		"BroadcastSetting": 3,
		"SupportedConnections": []map[string]any{{
			"ConnectionType": 7,
			"NetherNetId":    "1234567890",
			"PmsgId":         uuid.NewString(),
		}},
	}
	if change != nil {
		change(props)
	}
	raw, _ := json.Marshal(props)
	var a mpsd.ActivityHandle
	a.ID = uuid.New()
	a.OwnerXUID = host + "-xuid"
	a.CustomProperties = raw
	a.RelatedInfo = &mpsd.ActivityHandleRelatedInfo{JoinRestriction: "followed", MaxMembersCount: 8}
	return a
}

func TestFromActivities(t *testing.T) {
	// the texts below are compared in English
	locale.SetLanguage("en")
	closed := activity("Carol", "Closed World", nil)
	closed.RelatedInfo.Closed = true
	inviteOnly := activity("Dave", "Secret Base", func(m map[string]any) {
		m["Joinability"] = "invite_only"
		m["BroadcastSetting"] = 1
	})
	acts := []mpsd.ActivityHandle{
		activity("Zed", "Zed's Survival", nil),
		activity("Bob", "Full House", func(m map[string]any) { m["MemberCount"] = 8 }),
		closed,
		inviteOnly,
		activity("Eve", "Old Times", func(m map[string]any) { m["protocol"] = 100; m["version"] = "1.20.0" }),
		activity("Frank", "Lan Party", func(m map[string]any) { m["SupportedConnections"] = []any{} }),
		activity("Me", "My Own World", nil),
		activity("Ann", "", nil),
		// a connection type p2p doesn't know: can't be decoded, left out
		activity("Gus", "Odd", func(m map[string]any) {
			m["SupportedConnections"] = []map[string]any{{"ConnectionType": 4, "NetherNetId": "1"}}
		}),
	}
	worlds := FromActivities(acts, "Me-xuid", supported)

	want := []struct {
		host   string
		status Status
	}{
		{"Ann", Joinable}, {"Zed", Joinable},
		{"Bob", Full}, {"Carol", Closed}, {"Dave", InviteOnly}, {"Eve", OtherVersion}, {"Frank", NoConnection},
	}
	if len(worlds) != len(want) {
		for _, w := range worlds {
			t.Logf("%s %q %v", w.Host, w.Name, w.Status)
		}
		t.Fatalf("got %d worlds, want %d", len(worlds), len(want))
	}
	for i, w := range want {
		if worlds[i].Host != w.host || worlds[i].Status != w.status {
			t.Errorf("world %d: %s %v, want %s %v", i, worlds[i].Host, worlds[i].Status, w.host, w.status)
		}
	}
	if CountJoinable(worlds) != 2 {
		t.Errorf("CountJoinable = %d", CountJoinable(worlds))
	}

	zed := worlds[1]
	if zed.Address() != "xbox:"+acts[0].ID.String() {
		t.Errorf("address %q", zed.Address())
	}
	if zed.Title() != "Zed's Survival" || worlds[0].Title() != "Ann" {
		t.Errorf("titles %q %q", zed.Title(), worlds[0].Title())
	}
	if zed.PlayersText() != "Players: 2/8" || zed.StatusText() != "" {
		t.Errorf("players %q status %q", zed.PlayersText(), zed.StatusText())
	}
	for _, w := range worlds[2:] {
		if w.StatusText() == "" {
			t.Errorf("%s: no reason given", w.Host)
		}
	}
	if got := (&UnjoinableError{World: worlds[2]}).Error(); got != "Can't join Bob's world Full House: Full" {
		t.Errorf("unjoinable error %q", got)
	}
	if ErrGone.Error() == "" || !errors.Is(ErrGone, ErrGone) {
		t.Error("ErrGone")
	}
}

// fakeAccounts lets a test sign in and out and counts the checks made.
type fakeAccounts struct {
	key    atomic.Value
	checks atomic.Int32
	worlds []World
	err    error
}

func (f *fakeAccounts) current() (any, ListFunc) {
	key, _ := f.key.Load().(string)
	if key == "" {
		return nil, nil
	}
	return key, func(ctx context.Context) ([]World, error) {
		f.checks.Add(1)
		return f.worlds, f.err
	}
}

func newTestWatcher(f *fakeAccounts) (*Watcher, chan struct{}) {
	changed := make(chan struct{}, 16)
	w := &Watcher{
		Current:  f.current,
		Interval: time.Minute,
		MinGap:   time.Hour,
		Timeout:  time.Second,
		OnChange: func() { changed <- struct{}{} },
	}
	return w, changed
}

// waitChecked waits for a check to finish.
func waitChecked(t *testing.T, w *Watcher) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := w.Snapshot(); !s.Checking && !s.Checked.IsZero() {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("check never finished")
	return Snapshot{}
}

func TestWatcher(t *testing.T) {
	f := &fakeAccounts{worlds: []World{{Host: "Zed", Status: Joinable}, {Host: "Bob", Status: Full}}}
	w, changed := newTestWatcher(f)
	now := time.Now()

	// signed out: nothing is checked
	if next := w.Tick(now); !next.IsZero() || f.checks.Load() != 0 || w.Snapshot().SignedIn {
		t.Fatal("checked while signed out")
	}

	f.key.Store("alice")
	next := w.Tick(now)
	if !w.Snapshot().SignedIn || next.Sub(now) != time.Minute {
		t.Fatalf("first tick: next in %s", next.Sub(now))
	}
	s := waitChecked(t, w)
	if s.Joinable() != 1 || len(s.Worlds) != 2 || f.checks.Load() != 1 {
		t.Fatalf("after first check: joinable=%d worlds=%d checks=%d", s.Joinable(), len(s.Worlds), f.checks.Load())
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("OnChange wasn't called")
	}

	// not due yet; then due
	w.Tick(time.Now().Add(30 * time.Second))
	w.Refresh() // within MinGap
	if f.checks.Load() != 1 {
		t.Fatalf("checked again too soon: %d", f.checks.Load())
	}
	w.Tick(time.Now().Add(2 * time.Minute))
	waitChecked(t, w)
	if f.checks.Load() != 2 {
		t.Fatalf("due check didn't run: %d", f.checks.Load())
	}

	// a failed check keeps the worlds it knew and says why
	f.err = errors.New("xbox live is down")
	w.MinGap = 0
	w.Refresh()
	s = waitChecked(t, w)
	if s.Err == nil || s.Joinable() != 1 {
		t.Fatalf("after failed check: err=%v joinable=%d", s.Err, s.Joinable())
	}

	// another account: what was found for the last one is gone
	f.key.Store("bob")
	if s := w.Snapshot(); len(s.Worlds) != 0 || !s.Checked.IsZero() || s.Err != nil {
		t.Fatalf("worlds kept across accounts: %+v", s)
	}
	f.key.Store("")
	if s := w.Snapshot(); s.SignedIn || len(s.Worlds) != 0 {
		t.Fatal("signed out but still has worlds")
	}
}
