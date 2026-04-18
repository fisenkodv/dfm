package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fisenkodv/dfm/app/config"
)

// recorder is a Reporter that captures messages for assertions.
type recorder struct {
	actions []string
	infos   []string
	warns   []string
}

func (r *recorder) Action(format string, args ...any) {
	r.actions = append(r.actions, fmt.Sprintf(format, args...))
}
func (r *recorder) Info(format string, args ...any) {
	r.infos = append(r.infos, fmt.Sprintf(format, args...))
}
func (r *recorder) Warn(format string, args ...any) {
	r.warns = append(r.warns, fmt.Sprintf(format, args...))
}

// sandbox creates a fake dotfiles repo + home directory under t.TempDir().
// Returns (baseDir, homeDir). HOME is set via t.Setenv so expand("~/...")
// resolves into the sandbox.
func sandbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(base, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	return base, home
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readLink(t *testing.T, path string) string {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	return got
}

func TestLink_CreatesFreshSymlink(t *testing.T) {
	base, home := sandbox(t)
	source := filepath.Join(base, "config", "nvim")
	writeFile(t, filepath.Join(source, "init.lua"), "vim.cmd('echo hi')")

	cfg := &config.Config{Directives: []config.Directive{
		{
			Kind: config.KindLink,
			Link: &config.Link{Entries: []config.LinkEntry{
				{Target: "~/.config/nvim", Options: linkPath("config/nvim")},
			}},
		},
	}}
	r := &recorder{}
	e := New(base, r)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if tally.LinksCreated != 1 {
		t.Errorf("LinksCreated = %d, want 1; warns=%v", tally.LinksCreated, r.warns)
	}
	got := readLink(t, filepath.Join(home, ".config/nvim"))
	if got != source {
		t.Errorf("link target = %q, want %q", got, source)
	}
}

func TestLink_IdempotentWhenAlreadyCorrect(t *testing.T) {
	base, home := sandbox(t)
	source := filepath.Join(base, "config", "nvim")
	writeFile(t, filepath.Join(source, "init.lua"), "x")

	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindLink,
		Link: &config.Link{Entries: []config.LinkEntry{
			{Target: "~/.config/nvim", Options: linkPath("config/nvim")},
		}},
	}}}
	e := New(base, &recorder{})
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	r := &recorder{}
	e2 := New(base, r)
	tally, err := e2.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.LinksOK != 1 || tally.LinksCreated != 0 {
		t.Errorf("second apply: tally=%+v warns=%v", tally, r.warns)
	}
	// Target still correct.
	if readLink(t, filepath.Join(home, ".config/nvim")) != source {
		t.Errorf("link lost")
	}
}

func TestLink_RelinksStaleSymlinkWhenRelinkTrue(t *testing.T) {
	base, home := sandbox(t)
	good := filepath.Join(base, "config", "good")
	bad := filepath.Join(base, "config", "bad")
	writeFile(t, filepath.Join(good, "f"), "g")
	writeFile(t, filepath.Join(bad, "f"), "b")
	linkAt := filepath.Join(home, ".config/app")
	_ = os.MkdirAll(filepath.Dir(linkAt), 0o755)
	if err := os.Symlink(bad, linkAt); err != nil {
		t.Fatal(err)
	}

	relink := true
	opts := linkPath("config/good")
	opts.Relink = &relink
	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindLink,
		Link: &config.Link{Entries: []config.LinkEntry{
			{Target: "~/.config/app", Options: opts},
		}},
	}}}
	r := &recorder{}
	e := New(base, r)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.LinksRelinked != 1 {
		t.Errorf("LinksRelinked = %d, want 1. warns=%v", tally.LinksRelinked, r.warns)
	}
	if readLink(t, linkAt) != good {
		t.Errorf("link not updated")
	}
}

func TestLink_RefusesStaleWithoutRelink(t *testing.T) {
	base, home := sandbox(t)
	good := filepath.Join(base, "config", "good")
	writeFile(t, filepath.Join(good, "f"), "g")
	linkAt := filepath.Join(home, ".config/app")
	_ = os.MkdirAll(filepath.Dir(linkAt), 0o755)
	if err := os.Symlink("/some/other/place", linkAt); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindLink,
		Link: &config.Link{Entries: []config.LinkEntry{
			{Target: "~/.config/app", Options: linkPath("config/good")},
		}},
	}}}
	r := &recorder{}
	e := New(base, r)
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(r.warns) == 0 {
		t.Errorf("expected a warning about stale link; warns=%v", r.warns)
	}
	if readLink(t, linkAt) != "/some/other/place" {
		t.Errorf("link was unexpectedly modified")
	}
}

func TestLink_BacksUpExistingRegularFile(t *testing.T) {
	base, home := sandbox(t)
	source := filepath.Join(base, "config", "zshrc.zsh")
	writeFile(t, source, "repo content")
	existing := filepath.Join(home, ".zshrc")
	writeFile(t, existing, "PREEXISTING")

	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindLink,
		Link: &config.Link{Entries: []config.LinkEntry{
			{Target: "~/.zshrc", Options: linkPath("config/zshrc.zsh")},
		}},
	}}}
	r := &recorder{}
	e := New(base, r)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.LinksBackedUp != 1 || tally.LinksCreated != 1 {
		t.Errorf("tally=%+v warns=%v", tally, r.warns)
	}
	// Now linked to repo file.
	if target := readLink(t, existing); target != source {
		t.Errorf("link target %q, want %q", target, source)
	}
	// Backup sits somewhere under ~/.dotfiles-backup/
	backupRoot := filepath.Join(home, ".dotfiles-backup")
	found := false
	_ = filepath.WalkDir(backupRoot, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".zshrc") {
			data, _ := os.ReadFile(p)
			if string(data) == "PREEXISTING" {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Errorf("backup of pre-existing .zshrc not found under %s", backupRoot)
	}
}

func TestShell_RunsCommandInBaseDir(t *testing.T) {
	base, _ := sandbox(t)
	marker := filepath.Join(base, "marker")
	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindShell,
		Shell: &config.Shell{Entries: []config.ShellEntry{
			{Command: "echo ok > marker", Description: "write marker"},
		}},
	}}}
	e := New(base, &recorder{})
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.ShellRun != 1 {
		t.Errorf("ShellRun=%d", tally.ShellRun)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if strings.TrimSpace(string(data)) != "ok" {
		t.Errorf("marker=%q", data)
	}
}

func TestShell_FailedCommandIsReported(t *testing.T) {
	base, _ := sandbox(t)
	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindShell,
		Shell: &config.Shell{Entries: []config.ShellEntry{
			{Command: "false"},
			{Command: "true"},
		}},
	}}}
	r := &recorder{}
	e := New(base, r)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("apply returned fatal error: %v", err)
	}
	if tally.ShellFailed != 1 || tally.ShellRun != 1 {
		t.Errorf("tally=%+v warns=%v", tally, r.warns)
	}
}

func TestClean_RemovesDeadLinkIntoBase(t *testing.T) {
	base, home := sandbox(t)
	gone := filepath.Join(base, "config", "gone")
	if err := os.Symlink(gone, filepath.Join(home, "dangling")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "not-in-base")
	if err := os.Symlink(outside, filepath.Join(home, "outside")); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Directives: []config.Directive{{
		Kind:  config.KindClean,
		Clean: &config.Clean{Entries: []config.CleanEntry{{Target: "~"}}},
	}}}
	r := &recorder{}
	e := New(base, r)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.Cleaned != 1 {
		t.Errorf("Cleaned=%d, want 1 (only the in-base dangler); warns=%v", tally.Cleaned, r.warns)
	}
	if _, err := os.Lstat(filepath.Join(home, "dangling")); !os.IsNotExist(err) {
		t.Errorf("in-base dead link should be gone: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, "outside")); err != nil {
		t.Errorf("out-of-base dead link should remain: %v", err)
	}
}

func TestCreate_MakesDirectory(t *testing.T) {
	_, home := sandbox(t)
	mode := uint32(0o700)
	cfg := &config.Config{Directives: []config.Directive{{
		Kind:   config.KindCreate,
		Create: &config.Create{Entries: []config.CreateEntry{{Path: "~/secrets", Mode: &mode}}},
	}}}
	e := New(home, &recorder{})
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.Created != 1 {
		t.Errorf("Created=%d", tally.Created)
	}
	fi, err := os.Stat(filepath.Join(home, "secrets"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("not a dir")
	}
	if got := fi.Mode().Perm(); got != 0o700 {
		t.Errorf("mode = %o, want 0700", got)
	}
}

func TestDefaults_AppliedToLaterDirectives(t *testing.T) {
	base, home := sandbox(t)
	writeFile(t, filepath.Join(base, "config", "app", "f"), "x")
	stale := filepath.Join(home, ".config/app")
	_ = os.MkdirAll(filepath.Dir(stale), 0o755)
	if err := os.Symlink("/wrong", stale); err != nil {
		t.Fatal(err)
	}

	trueP := true
	cfg := &config.Config{Directives: []config.Directive{
		{
			Kind:     config.KindDefaults,
			Defaults: &config.Defaults{Link: &config.LinkOptions{Relink: &trueP}},
		},
		{
			Kind: config.KindLink,
			Link: &config.Link{Entries: []config.LinkEntry{
				{Target: "~/.config/app", Options: linkPath("config/app")},
			}},
		},
	}}
	e := New(base, &recorder{})
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tally.LinksRelinked != 1 {
		t.Errorf("defaults not applied: %+v", tally)
	}
}

// linkPath returns a LinkOptions with Path set, the common test shorthand.
func linkPath(p string) config.LinkOptions {
	s := p
	return config.LinkOptions{Path: &s}
}
