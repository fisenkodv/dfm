package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParse_RealProfiles ensures the user's actual profile files parse
// cleanly against this codebase. Skips when the dotfiles repo is not
// available (CI may run this repo in isolation).
func TestParse_RealProfiles(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	dir := filepath.Join(home, "Downloads", "dotfiles", "profiles")
	matches, err := filepath.Glob(filepath.Join(dir, "*.conf.yaml"))
	if err != nil || len(matches) == 0 {
		t.Skipf("no profiles at %s: %v", dir, err)
	}
	for _, m := range matches {
		t.Run(filepath.Base(m), func(t *testing.T) {
			cfg, err := Load(m)
			if err != nil {
				t.Fatalf("Load(%s): %v", m, err)
			}
			if len(cfg.Directives) == 0 {
				t.Errorf("empty directives")
			}
		})
	}
}
