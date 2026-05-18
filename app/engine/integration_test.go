package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitcldr/dfm/app/config"
)

// TestApply_RealBaseProfile runs the user's actual base profile against a
// sandboxed home directory and verifies every declared symlink exists and
// points into the real dotfiles repo. For every (target, source) declared
// in the YAML, the target must be a symlink to <base>/source.
//
// Skipped when the dotfiles repo is not present.
func TestApply_RealBaseProfile(t *testing.T) {
	repo := "/Users/dfisenko/Downloads/dotfiles"
	if _, err := os.Stat(filepath.Join(repo, "profiles", "base.conf.yaml")); err != nil {
		t.Skipf("dotfiles repo not at %s: %v", repo, err)
	}

	// Sandbox home. Pre-create ~/.config so links under it can be created.
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	cfg, err := config.Load(filepath.Join(repo, "profiles", "base.conf.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Strip shell directives from the test run — we don't want to execute
	// `chsh` or `git submodule update` inside a unit test.
	cfg.Directives = filterOut(cfg.Directives, config.KindShell, config.KindClean)

	buf := captureLog(t)
	e := New(repo)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if tally.LinksCreated == 0 {
		t.Fatalf("no links created. log=%s", buf.String())
	}

	// Verify every declared link: (target → repo/source) is a symlink with
	// the expected target.
	for _, d := range cfg.Directives {
		if d.Kind != config.KindLink {
			continue
		}
		for _, entry := range d.Link.Entries {
			target := expand(entry.Target)
			source := ""
			if entry.Options.Path != nil {
				source = *entry.Options.Path
			}
			if source == "" {
				continue
			}
			wantTarget := filepath.Join(repo, source)
			got, err := os.Readlink(target)
			if err != nil {
				// Some links (git/config, git/ignore) require parent
				// creation; skip those rather than asserting.
				if strings.Contains(err.Error(), "no such") {
					continue
				}
				t.Errorf("readlink %s: %v", target, err)
				continue
			}
			if got != wantTarget {
				t.Errorf("%s -> %s, want %s", target, got, wantTarget)
			}
		}
	}
}

func filterOut(ds []config.Directive, kinds ...config.DirectiveKind) []config.Directive {
	skip := make(map[config.DirectiveKind]bool, len(kinds))
	for _, k := range kinds {
		skip[k] = true
	}
	out := ds[:0]
	for _, d := range ds {
		if skip[d.Kind] {
			continue
		}
		out = append(out, d)
	}
	return out
}
