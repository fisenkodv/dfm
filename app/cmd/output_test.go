package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitcldr/dfm/app/iostreams"
)

// newTestIO wires up a pair of buffers and returns an IOStreams suitable for
// command tests (no color, no TTY).
func newTestIO() (ios *iostreams.IOStreams, out *bytes.Buffer, errOut *bytes.Buffer) {
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	ios = iostreams.NewTest(out, errOut)
	return
}

// ── completion ────────────────────────────────────────────────────────────────

func TestCompletion_OutputGoesToStdout(t *testing.T) {
	ios, out, errOut := newTestIO()
	c := &CompletionCmd{}
	c.SetIO(ios)
	c.Args.Shell = "bash"

	if err := c.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() == 0 {
		t.Error("completion script must go to Out (stdout)")
	}
	if errOut.Len() != 0 {
		t.Errorf("completion script must not go to ErrOut (stderr), got: %q", errOut)
	}
	if !strings.Contains(out.String(), "dfm") {
		t.Errorf("completion output missing expected content, got: %q", out)
	}
}

func TestCompletion_AllShells_GoToStdout(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			ios, out, errOut := newTestIO()
			c := &CompletionCmd{}
			c.SetIO(ios)
			c.Args.Shell = shell

			if err := c.Execute(nil); err != nil {
				t.Fatalf("Execute(%s): %v", shell, err)
			}
			if out.Len() == 0 {
				t.Errorf("%s: completion must go to Out", shell)
			}
			if errOut.Len() != 0 {
				t.Errorf("%s: completion must not go to ErrOut, got: %q", shell, errOut)
			}
		})
	}
}

func TestCompletion_QuietModePreservesOutput(t *testing.T) {
	ios, out, errOut := newTestIO()
	ios.SetQuiet()

	c := &CompletionCmd{}
	c.SetIO(ios)
	c.Args.Shell = "fish"

	if err := c.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() == 0 {
		t.Error("--quiet must not suppress completion script (data, not progress)")
	}
	if errOut.Len() != 0 {
		t.Errorf("no output expected on ErrOut for completion, got: %q", errOut)
	}
}

func TestCompletion_UnknownShell_ReturnsError(t *testing.T) {
	ios, _, _ := newTestIO()
	c := &CompletionCmd{}
	c.SetIO(ios)
	c.Args.Shell = "powershell"

	if err := c.Execute(nil); err == nil {
		t.Error("unknown shell must return an error")
	}
}

// ── list ──────────────────────────────────────────────────────────────────────

func TestList_WithProfiles_GoesToStdout(t *testing.T) {
	base := t.TempDir()
	profilesDir := filepath.Join(base, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"base", "work"} {
		p := filepath.Join(profilesDir, name+".conf.yaml")
		if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ios, out, errOut := newTestIO()
	c := &ListCmd{}
	c.SetIO(ios)
	c.globals.BaseDir = base

	if err := c.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("list must not go to ErrOut (stderr), got: %q", errOut)
	}
	for _, want := range []string{"base", "work"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list output missing %q, got: %q", want, out)
		}
	}
}

func TestList_QuietPreservesOutput(t *testing.T) {
	base := t.TempDir()
	profilesDir := filepath.Join(base, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "base.conf.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ios, out, _ := newTestIO()
	ios.SetQuiet()
	c := &ListCmd{}
	c.SetIO(ios)
	c.globals.BaseDir = base

	if err := c.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "base") {
		t.Errorf("--quiet must not suppress list output (data), got: %q", out)
	}
}

// ── status ────────────────────────────────────────────────────────────────────

func TestStatus_NoState_GoesToStdout(t *testing.T) {
	// Isolate state: point XDG_STATE_HOME at a fresh temp dir with no state file.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	ios, out, errOut := newTestIO()
	c := &StatusCmd{}
	c.SetIO(ios)

	if err := c.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() == 0 {
		t.Error("status output (including 'no profiles applied') must go to Out (stdout)")
	}
	if errOut.Len() != 0 {
		t.Errorf("status must not write to ErrOut (stderr), got: %q", errOut)
	}
}

// ── diff ──────────────────────────────────────────────────────────────────────

func TestDiffOutput_GoesToStdout(t *testing.T) {
	ios, out, errOut := newTestIO()

	printDiff(ios, nil) // nil actions → "no changes"

	if out.Len() == 0 {
		t.Error("diff output must go to Out (stdout)")
	}
	if errOut.Len() != 0 {
		t.Errorf("diff output must not go to ErrOut (stderr), got: %q", errOut)
	}
	if !strings.Contains(out.String(), "No changes") {
		t.Errorf("expected 'No changes', got: %q", out)
	}
}

func TestDiffOutput_QuietPreserved(t *testing.T) {
	ios, out, _ := newTestIO()
	ios.SetQuiet()

	printDiff(ios, nil)

	if !strings.Contains(out.String(), "No changes") {
		t.Errorf("diff output must still appear under --quiet (data, not progress), got: %q", out)
	}
}

// ── doctor under quiet ────────────────────────────────────────────────────────

func TestQuiet_DoctorDiagnosticsVisible(t *testing.T) {
	// Diagnostics must bypass the quiet gate.
	errOut := &bytes.Buffer{}
	ios := iostreams.NewTest(io.Discard, errOut)
	ios.SetQuiet()

	ios.DoctorFail("2 link(s) need attention")
	ios.DoctorItem("missing: ~/.vimrc")

	if errOut.Len() == 0 {
		t.Error("DoctorFail/DoctorItem must be visible on ErrOut under --quiet")
	}
	if !strings.Contains(errOut.String(), "need attention") {
		t.Errorf("DoctorFail message missing from ErrOut, got: %q", errOut)
	}
}

// ── IOSetter contract ─────────────────────────────────────────────────────────

func TestIOSetter_FallbackToRealStreams(t *testing.T) {
	c := &CompletionCmd{}
	// No SetIO call — IO() must return a non-nil fallback.
	ios := c.IO()
	if ios == nil {
		t.Fatal("IO() must never return nil")
	}
	if ios.Out == nil || ios.ErrOut == nil {
		t.Error("fallback IOStreams must have non-nil Out and ErrOut")
	}
}

// ── combined quiet + data routing ────────────────────────────────────────────

func TestQuiet_ProgressSuppressedDataFlows(t *testing.T) {
	ios, out, errOut := newTestIO()
	ios.SetQuiet()

	ios.ListItem("base")   // data → Out
	ios.Applying("/x")     // progress → suppressed
	ios.DoctorFail("oops") // diagnostic → ErrOut

	if !strings.Contains(out.String(), "base") {
		t.Errorf("data (ListItem) must survive --quiet, got out=%q", out)
	}
	if strings.Contains(errOut.String(), "applying") {
		t.Errorf("progress (Applying) must be suppressed by --quiet, got errOut=%q", errOut)
	}
	if !strings.Contains(errOut.String(), "oops") {
		t.Errorf("diagnostics (DoctorFail) must survive --quiet, got errOut=%q", errOut)
	}
}
