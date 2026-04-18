package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fisenkodv/dfm/app/cond"
	"github.com/fisenkodv/dfm/app/config"
)

// TestWhen_SkipsUnmatchedDirectives verifies that directives whose `when:`
// evaluates false are skipped entirely (no actions recorded).
func TestWhen_SkipsUnmatchedDirectives(t *testing.T) {
	base, home := sandbox(t)
	writeFile(t, filepath.Join(base, "file"), "x")

	cfg := &config.Config{Directives: []config.Directive{
		{
			Kind: config.KindLink,
			When: `os == "nope"`,
			Link: &config.Link{Entries: []config.LinkEntry{
				{Target: "~/skipped", Options: linkPath("file")},
			}},
		},
		{
			Kind: config.KindLink,
			When: `os == "testos"`,
			Link: &config.Link{Entries: []config.LinkEntry{
				{Target: "~/kept", Options: linkPath("file")},
			}},
		},
	}}

	e := New(base, &recorder{})
	e.Cond = cond.Context{OS: "testos", Arch: "x", Hostname: "h"}
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(home, "skipped")); !os.IsNotExist(err) {
		t.Errorf("skipped link materialised: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, "kept")); err != nil {
		t.Errorf("kept link missing: %v", err)
	}
}
