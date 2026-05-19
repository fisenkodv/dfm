// Package iostreams manages the three standard I/O streams for a command
// invocation and provides styled print helpers used by every subcommand.
package iostreams

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// IOStreams holds the standard streams for one command invocation.
//
//   - Out    receives data output (lists, completion scripts) — safe to pipe.
//   - ErrOut receives both progress and diagnostics. Progress is gated by the
//     quiet flag; diagnostics (doctor, warnings) always write through.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	// origOut/origErrOut are the writers passed at construction time. They
	// are kept so that SetColorPolicy("auto") can recompute TTY detection
	// from the real underlying streams even after a prior policy override.
	origOut    io.Writer
	origErrOut io.Writer

	quiet    bool // suppress progress output; diagnostics are unaffected
	outColor bool // whether Out supports ANSI color
	errColor bool // whether ErrOut supports ANSI color
}

// New returns an IOStreams wired to the real os streams with TTY- and
// environment-aware color detection.
func New() *IOStreams {
	ios := &IOStreams{
		In:         os.Stdin,
		Out:        os.Stdout,
		ErrOut:     os.Stderr,
		origOut:    os.Stdout,
		origErrOut: os.Stderr,
	}
	ios.outColor = computeColor(ios.Out)
	ios.errColor = computeColor(ios.ErrOut)
	return ios
}

// NewTest returns an IOStreams with custom writers and colors disabled.
// Intended for tests; quiet defaults to false so callers can toggle it.
func NewTest(out, errOut io.Writer) *IOStreams {
	return &IOStreams{
		In:         strings.NewReader(""),
		Out:        out,
		ErrOut:     errOut,
		origOut:    out,
		origErrOut: errOut,
		quiet:      false,
		outColor:   false,
		errColor:   false,
	}
}

// Discard returns an IOStreams that silently drops all output.
// Returned by engine.io() when no IOStreams has been injected (e.g. in tests).
func Discard() *IOStreams {
	return &IOStreams{
		In:         os.Stdin,
		Out:        io.Discard,
		ErrOut:     io.Discard,
		origOut:    io.Discard,
		origErrOut: io.Discard,
	}
}

// SetQuiet marks the streams as quiet: progress helpers become no-ops, but
// diagnostics (DoctorFail, DoctorItem, warnings) continue writing to ErrOut.
// Out is never affected — data output (lists, completion) always flows.
func (ios *IOStreams) SetQuiet() {
	ios.quiet = true
}

// SetColorPolicy overrides the TTY-detected color setting.
// policy must be "auto", "always", or "never".
// "auto" recomputes TTY detection from the original writers passed at
// construction, so it correctly undoes a prior "always" or "never" override.
func (ios *IOStreams) SetColorPolicy(policy string) {
	switch policy {
	case "always":
		ios.outColor = true
		ios.errColor = true
	case "never":
		ios.outColor = false
		ios.errColor = false
	case "auto":
		ios.outColor = computeColor(ios.origOut)
		ios.errColor = computeColor(ios.origErrOut)
	}
}

// IsQuiet reports whether quiet mode is active.
func (ios *IOStreams) IsQuiet() bool { return ios.quiet }

// OutColorEnabled reports whether Out is color-capable.
func (ios *IOStreams) OutColorEnabled() bool { return ios.outColor }

// ErrColorEnabled reports whether ErrOut is color-capable.
func (ios *IOStreams) ErrColorEnabled() bool { return ios.errColor }

// computeColor decides whether a writer should receive ANSI sequences.
// Environment variables take precedence over TTY detection:
//
//	NO_COLOR (any value)  → never (https://no-color.org/)
//	TERM=dumb             → never
//	CLICOLOR_FORCE=1      → always
//	otherwise             → TTY detection
func computeColor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") == "1" {
		return true
	}
	return isTTY(w)
}

// isTTY reports whether w is an interactive terminal.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// progressf writes a formatted line to ErrOut. It is a no-op when quiet.
// All progress helpers (Linked, Done, Applying, …) route through here so
// quiet suppression is enforced in one place.
func (ios *IOStreams) progressf(format string, args ...any) {
	if ios.quiet {
		return
	}
	fmt.Fprintf(ios.ErrOut, format, args...)
}

// cs (colorSprint) applies c only when enabled; returns s unchanged otherwise.
func cs(c *color.Color, s string, enabled bool) string {
	if !enabled {
		return s
	}
	return c.Sprint(s)
}

// ── style atoms ──────────────────────────────────────────────────────────────

var (
	boldColor      = color.New(color.Bold)
	boldGreenColor = color.New(color.Bold, color.FgGreen)
	boldWhiteColor = color.New(color.Bold, color.FgHiWhite)
	boldCyanColor  = color.New(color.Bold, color.FgCyan)
	yellowColor    = color.New(color.FgYellow)
	whiteColor     = color.New(color.FgHiWhite)
	dimColor       = color.New(color.FgHiBlack)
	hiRedColor     = color.New(color.FgHiRed)
)

// ── engine progress (→ ErrOut, gated by quiet) ───────────────────────────────

// Linked reports that a new symlink was created from→to.
func (ios *IOStreams) Linked(from, to string) {
	e := ios.errColor
	ios.progressf("%s %s %s %s\n",
		cs(boldGreenColor, "Linked", e), cs(whiteColor, from, e),
		cs(dimColor, "→", e), cs(dimColor, to, e))
}

// Relinked reports that a stale symlink was replaced with from→to.
func (ios *IOStreams) Relinked(from, to string) {
	e := ios.errColor
	ios.progressf("%s %s %s %s\n",
		cs(yellowColor, "Relinked", e), cs(whiteColor, from, e),
		cs(dimColor, "→", e), cs(dimColor, to, e))
}

// LinkOK reports that an existing symlink already points at the correct target.
func (ios *IOStreams) LinkOK(from, to string) {
	e := ios.errColor
	ios.progressf("%s %s %s %s\n",
		cs(boldColor, "Link exists", e), cs(dimColor, from, e),
		cs(whiteColor, "→", e), cs(dimColor, to, e))
}

// BackedUp reports that a pre-existing file was moved from→to before linking.
func (ios *IOStreams) BackedUp(from, to string) {
	e := ios.errColor
	ios.progressf("%s %s %s %s\n",
		cs(yellowColor, "backed up", e), cs(whiteColor, from, e),
		cs(dimColor, "→", e), cs(dimColor, to, e))
}

// RemovedDeadLink reports that a dangling symlink pointing from→to was removed.
func (ios *IOStreams) RemovedDeadLink(from, to string) {
	e := ios.errColor
	ios.progressf("%s %s %s %s\n",
		cs(yellowColor, "Removed", e), cs(whiteColor, from, e),
		cs(dimColor, "→", e), cs(dimColor, to, e))
}

// PathExists reports that a create target already exists and was skipped.
func (ios *IOStreams) PathExists(path string) {
	e := ios.errColor
	ios.progressf("%s %s\n", cs(boldColor, "Path exists", e), cs(dimColor, path, e))
}

// Created reports that a directory was created at path.
func (ios *IOStreams) Created(path string) {
	e := ios.errColor
	ios.progressf("%s %s\n", cs(boldGreenColor, "Created", e), cs(whiteColor, path, e))
}

// Applying reports that a profile at path is being applied.
func (ios *IOStreams) Applying(path string) {
	e := ios.errColor
	ios.progressf("%s %s\n", cs(boldColor, "Applying", e), cs(dimColor, path, e))
}

// WouldApply reports that a profile at path would be applied in a dry run.
func (ios *IOStreams) WouldApply(path string) {
	e := ios.errColor
	ios.progressf("%s %s\n", cs(boldCyanColor, "Would apply", e), cs(whiteColor, path, e))
}

// ShellCmd prints a shell directive entry. quietOpt mirrors the directive's quiet field.
func (ios *IOStreams) ShellCmd(description, command string, quietOpt bool) {
	e := ios.errColor
	switch {
	case quietOpt && description != "":
		ios.progressf("%s\n", cs(boldColor, description, e))
	case description != "":
		ios.progressf("%s %s\n", cs(boldColor, description, e), cs(dimColor, "["+command+"]", e))
	default:
		ios.progressf("%s\n", cs(whiteColor, command, e))
	}
}

// ApplyResult carries totals for the Done line, decoupled from engine.Tally
// to avoid an import cycle.
type ApplyResult struct {
	LinksOK, Created, Relinked, BackedUp int
	ShellRun, ShellFailed                int
	Cleaned, Dirs                        int
}

// Done prints the apply/dry-run summary line.
func (ios *IOStreams) Done(dryRun bool, r ApplyResult) {
	e := ios.errColor
	verb := cs(boldWhiteColor, "Done:", e)
	if dryRun {
		verb = cs(boldCyanColor, "dry-run", e)
	}
	sep := ", "
	ios.progressf("%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s\n",
		verb,
		" ", statCount(r.LinksOK, "ok", e),
		sep, statCount(r.Created, "created", e),
		sep, statCount(r.Relinked, "relinked", e),
		sep, statCount(r.BackedUp, "backed up", e),
		sep, shellCount(r.ShellRun, r.ShellFailed, e),
		sep, statCount(r.Cleaned, "cleaned", e),
		sep, statCount(r.Dirs, "dirs", e),
	)
}

// ── diagnostics (→ ErrOut, always visible regardless of quiet) ───────────────

// DoctorFail writes a plain diagnostic message (e.g. "no state found") to ErrOut.
func (ios *IOStreams) DoctorFail(msg string) {
	fmt.Fprintf(ios.ErrOut, "%s\n", cs(whiteColor, msg, ios.errColor))
}

// DoctorDone prints the inline doctor summary line, styled like Done.
// ok and problems are the checked and failed link counts respectively.
func (ios *IOStreams) DoctorDone(ok, problems int) {
	e := ios.errColor
	sep := ", "
	fmt.Fprintf(ios.ErrOut, "%s %s%s%s\n",
		cs(boldWhiteColor, "Doctor:", e),
		statCount(ok, "ok", e),
		sep, problemCount(problems, e),
	)
}

// DoctorItem writes one indented problem line under a DoctorDone summary.
func (ios *IOStreams) DoctorItem(problem string) {
	e := ios.errColor
	fmt.Fprintf(ios.ErrOut, "  %s\n", cs(dimColor, problem, e))
}

// ── status output (→ Out / stdout) ───────────────────────────────────────────

// StatusLine writes a "label value" pair to stdout.
func (ios *IOStreams) StatusLine(label, value string) {
	o := ios.outColor
	fmt.Fprintf(ios.Out, "%s %s\n", cs(boldColor, label, o), cs(dimColor, value, o))
}

// StatusLineWithMeta writes a "label value (meta)" triple to stdout.
func (ios *IOStreams) StatusLineWithMeta(label, value, meta string) {
	o := ios.outColor
	fmt.Fprintf(ios.Out, "%s %s %s\n",
		cs(boldColor, label, o), cs(dimColor, value, o), cs(whiteColor, meta, o))
}

// StatusEmpty writes a dim placeholder message when there is no status to show.
func (ios *IOStreams) StatusEmpty(msg string) {
	fmt.Fprintln(ios.Out, cs(dimColor, msg, ios.outColor))
}

// ── diff output (→ Out / stdout) ─────────────────────────────────────────────

// DiffHeader writes a section header with an item count for the diff output.
func (ios *IOStreams) DiffHeader(header string, n int) {
	o := ios.outColor
	fmt.Fprintf(ios.Out, "%s %s\n",
		cs(boldWhiteColor, header, o), cs(dimColor, "("+strconv.Itoa(n)+")", o))
}

// DiffAction writes one indented action line in the diff output.
func (ios *IOStreams) DiffAction(text string) {
	fmt.Fprintf(ios.Out, "  %s\n", cs(whiteColor, text, ios.outColor))
}

// DiffEmpty writes a "no changes" placeholder when the diff is empty.
func (ios *IOStreams) DiffEmpty() {
	fmt.Fprintln(ios.Out, cs(dimColor, "No changes", ios.outColor))
}

// ── list output (→ Out / stdout) ─────────────────────────────────────────────

// ListItem writes a single profile name to stdout, one per line.
func (ios *IOStreams) ListItem(name string) {
	fmt.Fprintln(ios.Out, name)
}

// ProfileList prints a styled profile list. Empty slice → "No profiles found".
// Non-empty → "Profiles: name1, name2" with white label/commas and dim names.
func (ios *IOStreams) ProfileList(names []string) {
	o := ios.outColor
	if len(names) == 0 {
		fmt.Fprintln(ios.Out, cs(dimColor, "No profiles found", o))
		return
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = cs(dimColor, n, o)
	}
	sep := cs(whiteColor, ",", o) + " "
	fmt.Fprintf(ios.Out, "%s %s\n", cs(whiteColor, "Profiles:", o), strings.Join(parts, sep))
}

// ── internal helpers ─────────────────────────────────────────────────────────

func statCount(n int, label string, colorOn bool) string {
	return cs(whiteColor, strconv.Itoa(n), colorOn) + " " + cs(dimColor, label, colorOn)
}

func problemCount(n int, colorOn bool) string {
	numColor := whiteColor
	if n > 0 {
		numColor = hiRedColor
	}
	return cs(numColor, strconv.Itoa(n), colorOn) + " " + cs(dimColor, "problems", colorOn)
}

func shellCount(run, failed int, colorOn bool) string {
	s := cs(whiteColor, strconv.Itoa(run), colorOn) + " " + cs(dimColor, "shell", colorOn)
	if failed > 0 {
		s += " " + cs(hiRedColor, "("+strconv.Itoa(failed)+" failed)", colorOn)
	}
	return s
}
