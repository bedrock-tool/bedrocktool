package popups

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/friendworlds"
	"github.com/google/uuid"
)

var friendWorlds = []friendworlds.World{
	{HandleID: uuid.New(), Host: "ZedCrafts", Name: "Zed's Survival", Members: 2, MaxMembers: 8, Status: friendworlds.Joinable},
	{HandleID: uuid.New(), Host: "Ann", Members: 1, Status: friendworlds.Joinable},
	{HandleID: uuid.New(), Host: "BobTheBuilder", Name: "Full House", Members: 8, MaxMembers: 8, Status: friendworlds.Full},
	{HandleID: uuid.New(), Host: "Eve", Name: "Old Times", Version: "1.20.0", Status: friendworlds.OtherVersion},
	{HandleID: uuid.New(), Host: "Dave", Name: "Secret Base", Status: friendworlds.InviteOnly},
}

// loadedWatcher is a watcher for a signed-in account whose check found
// worlds (or failed with err); signedIn false is no account.
func loadedWatcher(t *testing.T, signedIn bool, worlds []friendworlds.World, err error) *friendworlds.Watcher {
	t.Helper()
	w := &friendworlds.Watcher{
		Current: func() (any, friendworlds.ListFunc) {
			if !signedIn {
				return nil, nil
			}
			return "account", func(context.Context) ([]friendworlds.World, error) { return worlds, err }
		},
		Interval: time.Minute,
		MinGap:   time.Hour,
		Timeout:  time.Second,
	}
	if !signedIn {
		return w
	}
	w.Tick(time.Now())
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s := w.Snapshot(); !s.Checking && !s.Checked.IsZero() {
			return w
		}
		if time.Now().After(deadline) {
			t.Fatal("check never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// labels is all the text the popup shows.
func labels(r *input.Router) string {
	var out []string
	for _, n := range r.AppendSemantics(nil) {
		if n.Desc.Label != "" {
			out = append(out, n.Desc.Label)
		}
	}
	return strings.Join(out, "\n")
}

func TestFriendWorldsCloses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		signedIn bool
		worlds   []friendworlds.World
		err      error
		shows    string
	}{
		{"friend_worlds", true, friendWorlds, nil, "Zed's Survival"},
		{"friend_worlds_none", true, nil, nil, locale.Loc("gui_friends_none", nil)},
		{"friend_worlds_error", true, nil, errors.New("xbox live is down"), locale.Loc("gui_friends_error", locale.Strmap{"Err": "xbox live is down"})},
		{"friend_worlds_signed_out", false, nil, nil, locale.Loc("gui_friends_signed_out", nil)},
	} {
		p := NewFriendWorlds(nil, loadedWatcher(t, tc.signedIn, tc.worlds, tc.err), func(friendworlds.World) {
			t.Fatalf("%s: picked a world on Close", tc.name)
		}).(*FriendWorlds)
		p.g = &fakeGuim{}
		r := new(input.Router)
		frame(r, darkTheme(), p)
		if got := labels(r); !strings.Contains(got, tc.shows) {
			t.Errorf("%s: doesn't show %q:\n%s", tc.name, tc.shows, got)
		}
		closes(t, p, locale.Loc("gui_close", nil), tc.name)
	}
}

func TestFriendWorldsRows(t *testing.T) {
	p := NewFriendWorlds(nil, loadedWatcher(t, true, friendWorlds, nil), func(friendworlds.World) {}).(*FriendWorlds)
	p.g = &fakeGuim{}
	r := new(input.Router)
	frame(r, darkTheme(), p)
	all := labels(r)
	for _, want := range []string{
		"ZedCrafts · " + friendWorlds[0].PlayersText(), friendWorlds[1].PlayersText(), // gamertag and player count
		"BobTheBuilder · " + friendWorlds[2].StatusText(), friendWorlds[3].StatusText(), friendWorlds[4].StatusText(),
		locale.Loc("gui_friends_select", nil), locale.Loc("gui_refresh", nil),
	} {
		if !strings.Contains(all, want) {
			t.Errorf("doesn't show %q:\n%s", want, all)
		}
	}
	// only joinable worlds are buttons: one that can't be joined has nothing to click
	buttons := 0
	nodes := r.AppendSemantics(nil)
	for _, n := range nodes {
		if n.Desc.Class != semantic.Button {
			continue
		}
		for _, c := range nodes {
			if c.ParentID == n.ID && strings.Contains(c.Desc.Label, locale.Loc("gui_friends_select", nil)) {
				buttons++
			}
			if c.ParentID == n.ID && (c.Desc.Label == "Full House" || c.Desc.Label == "Old Times") {
				t.Errorf("%q is clickable but can't be joined", c.Desc.Label)
			}
		}
	}
	if buttons != 2 {
		t.Errorf("%d Select buttons, want 2", buttons)
	}
}

func TestFriendWorldsPicks(t *testing.T) {
	var picked friendworlds.World
	p := NewFriendWorlds(nil, loadedWatcher(t, true, friendWorlds, nil), func(w friendworlds.World) { picked = w }).(*FriendWorlds)
	g := &fakeGuim{}
	p.g = g
	th := darkTheme()
	r := new(input.Router)
	frame(r, th, p)
	click(r, buttonAt(t, r, "Zed's Survival"))
	frame(r, th, p)
	if picked.HandleID != friendWorlds[0].HandleID || len(g.closed) == 0 {
		t.Fatalf("clicking a world should pick it and close: picked=%q closed=%v", picked.Name, g.closed)
	}
	if picked.Address() != "xbox:"+friendWorlds[0].HandleID.String() {
		t.Fatalf("address %q", picked.Address())
	}
}

// Refresh checks again; opening the popup asked for a check already, which
// MinGap held back.
func TestFriendWorldsRefresh(t *testing.T) {
	checks := 0
	w := &friendworlds.Watcher{
		Current: func() (any, friendworlds.ListFunc) {
			return "account", func(context.Context) ([]friendworlds.World, error) {
				checks++
				return friendWorlds, nil
			}
		},
		Interval: time.Minute, MinGap: 0, Timeout: time.Second,
	}
	p := NewFriendWorlds(nil, w, func(friendworlds.World) {}).(*FriendWorlds)
	p.g = &fakeGuim{}
	th := darkTheme()
	r := new(input.Router)
	frame(r, th, p) // opening checks
	waitIdle(t, w)
	frame(r, th, p)
	click(r, buttonAt(t, r, locale.Loc("gui_refresh", nil)))
	frame(r, th, p)
	waitIdle(t, w)
	if checks != 2 {
		t.Fatalf("%d checks, want 2 (open, Refresh)", checks)
	}
}

func waitIdle(t *testing.T, w *friendworlds.Watcher) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for w.Snapshot().Checking {
		if time.Now().After(deadline) {
			t.Fatal("check never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
