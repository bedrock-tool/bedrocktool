// Package savedsettings keeps what the GUI was last started with, per mode,
// and the servers it connected to recently, so they don't need entering
// again on the next launch.
package savedsettings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MaxRecent is how many recent servers are kept.
const MaxRecent = 10

// Mode is what a mode (worlds, skins, packs) was last started with.
type Mode struct {
	Address string `json:"address,omitempty"`
	// Options are setting values by flag name, as the settings page reads
	// them ("true", "76", "normal"...).
	Options map[string]string `json:"options,omitempty"`
}

// Saved is the whole file.
type Saved struct {
	Modes map[string]Mode `json:"modes,omitempty"`
	// Recent are addresses, newest first: servers, realm:, experience:
	// and xbox: values.
	Recent []string `json:"recent_servers,omitempty"`
}

// Load reads the file at path. A missing file is no saved settings; so is a
// broken one, with the error to log.
func Load(path string) (*Saved, error) {
	s := &Saved{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return &Saved{}, err
	}
	// whatever was written by hand: same rules as AddRecent
	recent := s.Recent
	s.Recent = nil
	for i := len(recent) - 1; i >= 0; i-- {
		s.AddRecent(recent[i])
	}
	return s, nil
}

// Save writes the file at path, through a temporary file so a crash
// can't leave half of it.
func (s *Saved) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Mode is what mode was last started with, if it was.
func (s *Saved) Mode(mode string) (Mode, bool) {
	m, ok := s.Modes[mode]
	return m, ok
}

// SetMode remembers what mode was started with.
func (s *Saved) SetMode(mode string, m Mode) {
	if s.Modes == nil {
		s.Modes = make(map[string]Mode)
	}
	m.Address = strings.TrimSpace(m.Address)
	s.Modes[mode] = m
}

// AddRecent puts address first in the recent servers, once, keeping
// MaxRecent of them.
func (s *Saved) AddRecent(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		return
	}
	s.Recent = slices.DeleteFunc(s.Recent, func(a string) bool {
		return strings.EqualFold(a, address)
	})
	s.Recent = slices.Insert(s.Recent, 0, address)
	if len(s.Recent) > MaxRecent {
		s.Recent = s.Recent[:MaxRecent]
	}
}

// RemoveRecent takes address off the recent servers.
func (s *Saved) RemoveRecent(address string) {
	address = strings.TrimSpace(address)
	s.Recent = slices.DeleteFunc(s.Recent, func(a string) bool {
		return strings.EqualFold(a, address)
	})
}
