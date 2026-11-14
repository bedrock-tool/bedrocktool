package friendworlds

import (
	"context"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/sirupsen/logrus"
)

// ListFunc lists the friends' worlds of one account.
type ListFunc func(ctx context.Context) ([]World, error)

// Snapshot is what the Watcher knows right now.
type Snapshot struct {
	// SignedIn is false when no account is signed in; nothing is checked then.
	SignedIn bool
	// Checking is true while a check is running.
	Checking bool
	// Checked is when the last check finished; zero before the first.
	Checked time.Time
	// Worlds are from the last check that worked.
	Worlds []World
	// Err is why the last check failed, nil when it worked.
	Err error
}

// Joinable is how many worlds can be joined.
func (s Snapshot) Joinable() int {
	return CountJoinable(s.Worlds)
}

// Watcher keeps the signed-in account's friends' worlds up to date without
// blocking whoever asks: checks run in the background, at most one at a
// time, every Interval while someone keeps calling Tick.
type Watcher struct {
	// Current is the account signed in now (any value that tells accounts
	// apart) and how to list its friends' worlds; nil key when signed out.
	Current func() (key any, list ListFunc)
	// Interval is how often Tick checks again.
	Interval time.Duration
	// MinGap is the least time between two checks Refresh asks for.
	MinGap time.Duration
	// Timeout bounds one check.
	Timeout time.Duration
	// OnChange is called after a check starts or finishes, from any goroutine.
	OnChange func()

	mu      sync.Mutex
	key     any
	gen     int // bumped on account change; results of older checks are dropped
	running bool
	snap    Snapshot
}

// NewWatcher watches the account signed in to bedrocktool.
func NewWatcher() *Watcher {
	return &Watcher{
		Current: func() (any, ListFunc) {
			account := auth.Auth.Account()
			if account == nil {
				return nil, nil
			}
			return account, func(ctx context.Context) ([]World, error) {
				return List(ctx, account)
			}
		},
		Interval: 90 * time.Second,
		MinGap:   5 * time.Second,
		Timeout:  20 * time.Second,
	}
}

// Snapshot is a copy of what is known now.
func (w *Watcher) Snapshot() Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncAccount()
	return w.snapLocked()
}

func (w *Watcher) snapLocked() Snapshot {
	s := w.snap
	s.Worlds = append([]World(nil), s.Worlds...)
	return s
}

// syncAccount forgets what was found for an account that signed out or
// was swapped for another.
func (w *Watcher) syncAccount() ListFunc {
	key, list := w.Current()
	if key != w.key {
		w.key = key
		w.gen++
		w.running = false
		w.snap = Snapshot{SignedIn: key != nil}
	}
	return list
}

// Tick starts a check when one is due and returns when the next one will
// be. Call it while the worlds are on screen.
func (w *Watcher) Tick(now time.Time) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	list := w.syncAccount()
	if list == nil {
		return time.Time{}
	}
	next := w.snap.Checked.Add(w.Interval)
	if w.running {
		return now.Add(w.Interval)
	}
	if w.snap.Checked.IsZero() || !now.Before(next) {
		w.startLocked(list)
		return now.Add(w.Interval)
	}
	return next
}

// Refresh checks now, unless a check is running or one finished less than
// MinGap ago.
func (w *Watcher) Refresh() {
	w.mu.Lock()
	defer w.mu.Unlock()
	list := w.syncAccount()
	if list == nil || w.running {
		return
	}
	if !w.snap.Checked.IsZero() && time.Since(w.snap.Checked) < w.MinGap {
		return
	}
	w.startLocked(list)
}

func (w *Watcher) startLocked(list ListFunc) {
	w.running = true
	w.snap.Checking = true
	gen := w.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), w.Timeout)
		worlds, err := list(ctx)
		cancel()
		w.mu.Lock()
		if gen != w.gen {
			w.mu.Unlock()
			return
		}
		w.running = false
		w.snap.Checking = false
		w.snap.Checked = time.Now()
		w.snap.Err = err
		if err == nil {
			w.snap.Worlds = worlds
		} else {
			logrus.Debugf("checking friends' worlds: %s", err)
		}
		w.mu.Unlock()
		w.changed()
	}()
	w.changedAsync()
}

func (w *Watcher) changed() {
	if w.OnChange != nil {
		w.OnChange()
	}
}

// changedAsync tells OnChange without holding the lock up.
func (w *Watcher) changedAsync() {
	if w.OnChange != nil {
		go w.OnChange()
	}
}
