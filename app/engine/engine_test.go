package engine

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitcldr/dfm/app/config"
	"github.com/bitcldr/dfm/app/iostreams"
)

// captureLog redirects the standard logger to a buffer for the duration of
// the test. Returns the buffer so callers can assert on [WARN]/[INFO] lines.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	old := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return buf
}

// sandbox creates a fake dotfiles repo + home directory under t.TempDir().
// Returns (baseDir, homeDir). HOME is set via t.Setenv so expand("~/...")
// resolves into the sandbox.
func sandbox(t *testing.T) (baseDir, homeDir string) {
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
	buf := captureLog(t)
	e := New(base)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if tally.LinksCreated != 1 {
		t.Errorf("LinksCreated = %d, want 1; log=%s", tally.LinksCreated, buf.String())
	}

	got := readLink(t, filepath.Join(home, ".config", "nvim"))
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
	e := New(base)
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	buf := captureLog(t)
	e2 := New(base)
	tally, err := e2.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if tally.LinksOK != 1 || tally.LinksCreated != 0 {
		t.Errorf("second apply: tally=%+v log=%s", tally, buf.String())
	}

	// Target still correct.
	if readLink(t, filepath.Join(home, ".config", "nvim")) != source {
		t.Errorf("link lost")
	}
}

func TestLink_RelinksStaleSymlinkWhenRelinkTrue(t *testing.T) {
	base, home := sandbox(t)
	good := filepath.Join(base, "config", "good")
	bad := filepath.Join(base, "config", "bad")
	writeFile(t, filepath.Join(good, "f"), "g")
	writeFile(t, filepath.Join(bad, "f"), "b")
	linkAt := filepath.Join(home, ".config", "app")
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
	buf := captureLog(t)
	e := New(base)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if tally.LinksRelinked != 1 {
		t.Errorf("LinksRelinked = %d, want 1. log=%s", tally.LinksRelinked, buf.String())
	}

	if readLink(t, linkAt) != good {
		t.Errorf("link not updated")
	}
}

func TestLink_RefusesStaleWithoutRelink(t *testing.T) {
	base, home := sandbox(t)
	good := filepath.Join(base, "config", "good")
	writeFile(t, filepath.Join(good, "f"), "g")
	linkAt := filepath.Join(home, ".config", "app")
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
	buf := captureLog(t)
	e := New(base)
	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "[WARN]") {
		t.Errorf("expected a warning about stale link; log=%s", buf.String())
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
	buf := captureLog(t)
	e := New(base)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if tally.LinksBackedUp != 1 || tally.LinksCreated != 1 {
		t.Errorf("tally=%+v log=%s", tally, buf.String())
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
	e := New(base)
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
	buf := captureLog(t)
	e := New(base)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("apply returned fatal error: %v", err)
	}

	if tally.ShellFailed != 1 || tally.ShellRun != 2 {
		t.Errorf("tally=%+v log=%s", tally, buf.String())
	}
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Error("expected warning for failed command, got none")
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
	buf := captureLog(t)
	e := New(base)
	tally, err := e.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if tally.Cleaned != 1 {
		t.Errorf("Cleaned=%d, want 1 (only the in-base dangler); log=%s", tally.Cleaned, buf.String())
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
	e := New(home)
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
	stale := filepath.Join(home, ".config", "app")
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
	e := New(base)
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

func TestShell_SubprocessStdoutUsesIOStreams(t *testing.T) {
	base, _ := sandbox(t)
	out := &bytes.Buffer{}
	ios := iostreams.NewTest(out, io.Discard)

	trueVal := true
	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindShell,
		Shell: &config.Shell{Entries: []config.ShellEntry{
			{Command: "echo hello", Options: config.ShellOptions{Stdout: &trueVal}},
		}},
	}}}

	e := New(base)
	e.IO = ios

	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("subprocess stdout must go through IOStreams.Out, got: %q", out)
	}
}

func TestShell_GlobalQuietDoesNotSuppressSubprocessStreams(t *testing.T) {
	// Global --quiet suppresses dfm's own progress, not subprocess output.
	// Subprocess stdout/stderr are controlled only by the directive's own quiet option.
	base, _ := sandbox(t)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := iostreams.NewTest(out, errOut)
	ios.SetQuiet() // suppress dfm progress only

	trueVal := true
	cfg := &config.Config{Directives: []config.Directive{{
		Kind: config.KindShell,
		Shell: &config.Shell{Entries: []config.ShellEntry{
			{
				Command: "printf 'stdout-data' && printf 'stderr-data' >&2",
				Options: config.ShellOptions{Stdout: &trueVal, Stderr: &trueVal},
			},
		}},
	}}}

	e := New(base)
	e.IO = ios

	if _, err := e.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "stdout-data") {
		t.Errorf("subprocess stdout must flow through under --quiet, got out: %q", out)
	}
	if !strings.Contains(errOut.String(), "stderr-data") {
		t.Errorf("subprocess stderr must flow through under --quiet, got errOut: %q", errOut)
	}
	// dfm's own ShellCmd progress line must be suppressed.
	if strings.Contains(errOut.String(), "printf") {
		t.Errorf("dfm progress (ShellCmd) must be suppressed by --quiet, got errOut: %q", errOut)
	}
}
