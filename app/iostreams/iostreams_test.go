package iostreams

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// ── quiet behavior ────────────────────────────────────────────────────────────

func TestSetQuiet_DoesNotReplaceErrOut(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)
	ios.SetQuiet()

	// ErrOut must still be the original buffer, not io.Discard.
	if ios.ErrOut == io.Discard {
		t.Error("SetQuiet must not replace ErrOut — diagnostics must retain their destination")
	}
	if !ios.IsQuiet() {
		t.Error("IsQuiet must return true after SetQuiet")
	}
}

func TestSetQuiet_DoesNotAffectOut(t *testing.T) {
	out := &bytes.Buffer{}
	ios := NewTest(out, io.Discard)
	ios.SetQuiet()

	if ios.Out == io.Discard {
		t.Error("SetQuiet must not touch Out — data output (list, completion) must remain")
	}
}

func TestSetQuiet_SuppressesProgress(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)
	ios.SetQuiet()

	ios.Linked("/a", "/b")
	ios.Applying("/profile")
	ios.Done(false, ApplyResult{LinksOK: 1})

	if errOut.Len() != 0 {
		t.Errorf("quiet must suppress progress output, got: %q", errOut)
	}
}

func TestSetQuiet_DiagnosticsStillVisible(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)
	ios.SetQuiet()

	// Doctor diagnostics must bypass the quiet gate.
	ios.DoctorFail("2 link(s) need attention")
	ios.DoctorItem("missing: ~/.vimrc")
	ios.DoctorDone(0, 0)

	if errOut.Len() == 0 {
		t.Error("DoctorFail/DoctorItem/DoctorDone must write to ErrOut even under --quiet")
	}
	if !strings.Contains(errOut.String(), "need attention") {
		t.Errorf("DoctorFail message missing, got: %q", errOut)
	}
	if !strings.Contains(errOut.String(), "missing:") {
		t.Errorf("DoctorItem message missing, got: %q", errOut)
	}
}

func TestSetQuiet_DataStillFlows(t *testing.T) {
	out := &bytes.Buffer{}
	ios := NewTest(out, io.Discard)
	ios.SetQuiet()

	ios.ListItem("base")
	ios.StatusLine("last applied:", "base")

	if !strings.Contains(out.String(), "base") {
		t.Errorf("data output must survive --quiet, got: %q", out)
	}
}

// ── stream separation ─────────────────────────────────────────────────────────

func TestProgressGoesToErrOut(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := NewTest(out, errOut)

	ios.Linked("/home/.vimrc", "/dotfiles/vimrc")
	ios.Relinked("/home/.zshrc", "/dotfiles/zshrc")
	ios.Done(false, ApplyResult{LinksOK: 2})

	if out.Len() != 0 {
		t.Errorf("progress must not go to Out (stdout), got: %q", out)
	}
	if errOut.Len() == 0 {
		t.Error("progress must go to ErrOut (stderr)")
	}
}

func TestDataGoesToOut(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := NewTest(out, errOut)

	ios.ListItem("base")
	ios.StatusLine("links:", "5")
	ios.DiffHeader("+ links to create", 2)
	ios.DiffAction("/home/.vimrc -> /dotfiles/vimrc")
	ios.DiffEmpty()

	if out.Len() == 0 {
		t.Error("data output must go to Out (stdout)")
	}
	if errOut.Len() != 0 {
		t.Errorf("data output must not go to ErrOut (stderr), got: %q", errOut)
	}
}

func TestDiagnosticsGoToErrOut(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := NewTest(out, errOut)

	ios.DoctorFail("1 link(s) need attention")
	ios.DoctorItem("missing: ~/.vimrc")
	ios.DoctorDone(5, 0)

	if out.Len() != 0 {
		t.Errorf("diagnostics must not go to Out (stdout), got: %q", out)
	}
	if errOut.Len() == 0 {
		t.Error("diagnostics must go to ErrOut (stderr)")
	}
}

// ── color behavior ────────────────────────────────────────────────────────────

func TestNoColorWhenDisabled(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := NewTest(out, errOut) // outColor=false, errColor=false

	ios.Linked("/a", "/b")
	ios.Done(false, ApplyResult{LinksOK: 1})
	ios.DiffHeader("+ links to create", 1)
	ios.DoctorFail("problem")
	ios.StatusLine("links:", "3")

	for name, s := range map[string]string{"errOut": errOut.String(), "out": out.String()} {
		if strings.Contains(s, "\x1b[") {
			t.Errorf("ANSI sequences found in %s (non-TTY writer): %q", name, s)
		}
	}
}

// ── SetColorPolicy("auto") roundtrip ─────────────────────────────────────────

func TestSetColorPolicy_AutoRecomputes(t *testing.T) {
	// Force color on, then switch back to auto — auto must recompute from
	// the original (non-TTY) writers, giving false, not leaving it stuck at true.
	ios := NewTest(io.Discard, io.Discard)
	ios.SetColorPolicy("always")
	if !ios.OutColorEnabled() || !ios.ErrColorEnabled() {
		t.Fatal("always must enable color")
	}

	ios.SetColorPolicy("auto")
	// origOut/origErrOut are io.Discard (non-TTY) → color must be false.
	if ios.OutColorEnabled() || ios.ErrColorEnabled() {
		t.Error("SetColorPolicy(auto) must recompute from original writers, not leave prior override in place")
	}
}

// ── color env var behavior ────────────────────────────────────────────────────

// unsetenv removes key for the test duration, restoring it on cleanup.
// Unlike t.Setenv("KEY", ""), this truly removes the key so os.LookupEnv
// returns (_, false), which is required for NO_COLOR / CLICOLOR_FORCE tests.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestComputeColor_CLICOLORForce(t *testing.T) {
	unsetenv(t, "NO_COLOR")        // must be absent — presence wins over CLICOLOR_FORCE
	unsetenv(t, "TERM")            // must not be "dumb"
	t.Setenv("CLICOLOR_FORCE", "1")
	// Must enable color even for non-TTY writers (io.Discard).
	ios := New()
	if !ios.OutColorEnabled() || !ios.ErrColorEnabled() {
		t.Error("CLICOLOR_FORCE=1 must enable color regardless of TTY")
	}
}

func TestComputeColor_NOColor(t *testing.T) {
	// NO_COLOR overrides CLICOLOR_FORCE.
	unsetenv(t, "TERM")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	ios := New()
	if ios.OutColorEnabled() || ios.ErrColorEnabled() {
		t.Error("NO_COLOR must disable color and override CLICOLOR_FORCE")
	}
}

func TestComputeColor_TermDumb(t *testing.T) {
	unsetenv(t, "NO_COLOR")
	t.Setenv("TERM", "dumb")
	t.Setenv("CLICOLOR_FORCE", "0") // not "1", so won't force-enable
	ios := New()
	if ios.OutColorEnabled() || ios.ErrColorEnabled() {
		t.Error("TERM=dumb must disable color")
	}
}

func TestSetColorPolicy_Always(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)
	ios.SetColorPolicy("always")

	if !ios.ErrColorEnabled() {
		t.Error("SetColorPolicy(always) must enable errColor")
	}
}

func TestSetColorPolicy_Never(t *testing.T) {
	// Start with colors forced on.
	ios := &IOStreams{
		In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard,
		outColor: true, errColor: true,
	}
	ios.SetColorPolicy("never")

	if ios.OutColorEnabled() || ios.ErrColorEnabled() {
		t.Error("SetColorPolicy(never) must disable all color")
	}
}

// ── Discard constructor ───────────────────────────────────────────────────────

func TestDiscard_SuppressesBothStreams(t *testing.T) {
	ios := Discard()

	if ios.Out != io.Discard || ios.ErrOut != io.Discard {
		t.Error("Discard must set both Out and ErrOut to io.Discard")
	}
	// Must not panic.
	ios.Linked("/a", "/b")
	ios.DoctorFail("x")
	ios.ListItem("y")
}

// ── content sanity ────────────────────────────────────────────────────────────

func TestContentPresent(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)

	ios.Applying("/profiles/base.conf.yaml")
	ios.Linked("/home/.vimrc", "/dotfiles/vimrc")
	ios.Done(false, ApplyResult{LinksOK: 1, Created: 1})

	for _, want := range []string{"Applying", "Linked", "Done:"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("expected %q in ErrOut, got: %q", want, errOut)
		}
	}
}

func TestDryRunDone(t *testing.T) {
	errOut := &bytes.Buffer{}
	ios := NewTest(io.Discard, errOut)
	ios.Done(true, ApplyResult{Created: 3})

	if !strings.Contains(errOut.String(), "dry-run") {
		t.Errorf("expected 'dry-run' in summary, got: %q", errOut)
	}
}
