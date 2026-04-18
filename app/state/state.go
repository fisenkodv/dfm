// Package state persists and reads dfm's per-machine application state.
//
// The file lives at ~/.local/state/dfm/state.json (XDG default) and records
// which profiles were last applied, when, and which symlinks the engine
// created. `dfm status` reads it; `dfm doctor` uses it to verify drift.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is the on-disk representation.
type State struct {
	LastApplied []string  `json:"last_applied"`
	AppliedAt   time.Time `json:"applied_at"`
	Links       []Link    `json:"links"`
}

// Link records one symlink the engine created or confirmed. Paths are stored
// as provided (post tilde expansion) so `doctor` can re-stat them.
type Link struct {
	Target string `json:"target"`
	Source string `json:"source"`
}

// Path returns the absolute path of the state file, honoring XDG_STATE_HOME.
func Path() (string, error) {
	if p := os.Getenv("XDG_STATE_HOME"); p != "" {
		return filepath.Join(p, "dfm", "state.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("state: home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "dfm", "state.json"), nil
}

// Load reads the state file. A missing file returns (nil, nil) so callers
// can distinguish "never applied" from "corrupt".
func Load() (*State, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // path is derived from $HOME
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", p, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", p, err)
	}
	return &s, nil
}

// Save writes the state file, creating the parent directory as needed.
func Save(s *State) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("state: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil { //nolint:gosec // user-local config
		return fmt.Errorf("state: write %s: %w", p, err)
	}
	return nil
}
