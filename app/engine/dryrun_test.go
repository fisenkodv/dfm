package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitcldr/dfm/app/config"
)

// TestDryRun_RecordsWithoutMutating verifies that DryRun=true produces the
// same Actions that a real run would, but leaves the filesystem untouched.
func TestDryRun_RecordsWithoutMutating(t *testing.T) {
	base, home := sandbox(t)
	source := filepath.Join(base, "config", "nvim")
	writeFile(t, filepath.Join(source, "init.lua"), "x")

	cfg := &config.Config{Directives: []config.Directive{
		{
			Kind: config.KindShell,
			Shell: &config.Shell{Entries: []config.ShellEntry{
				{Command: "touch " + filepath.Join(home, "should-not-exist")},
			}},
		},
		{
			Kind: config.KindLink,
			Link: &config.Link{Entries: []config.LinkEntry{
				{Target: "~/.config/nvim", Options: linkPath("config/nvim")},
			}},
		},
	}}

	e := New(base, &recorder{})
	e.DryRun = true
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Actions recorded for both shell and link.
	kinds := map[ActionKind]int{}
	for _, a := range e.Actions {
		kinds[a.Kind]++
		if !a.DryRun {
			t.Errorf("action not marked dry-run: %+v", a)
		}
	}
	if kinds[ActionShellRun] != 1 {
		t.Errorf("shell action count: %d", kinds[ActionShellRun])
	}
	if kinds[ActionLinkCreate] != 1 {
		t.Errorf("link action count: %d", kinds[ActionLinkCreate])
	}

	// Filesystem must be pristine.
	if _, err := os.Lstat(filepath.Join(home, ".config", "nvim")); !os.IsNotExist(err) {
		t.Errorf("dry-run created a symlink: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, "should-not-exist")); !os.IsNotExist(err) {
		t.Errorf("dry-run executed a shell command: %v", err)
	}
}
