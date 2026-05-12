package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitcldr/dfm/app/config"
	"github.com/bitcldr/dfm/app/engine"
	"github.com/bitcldr/dfm/app/state"
)

// ApplyCmd applies one or more profiles to the user's home directory by
// iterating their directives in order. Profiles are loaded from
// <BaseDir>/profiles/<name>.conf.yaml unless an explicit --config path was
// given, in which case only that file is applied.
type ApplyCmd struct {
	base
	DryRun bool `long:"dry-run" description:"report planned changes without writing"`
	Args   struct {
		Profiles []string `positional-arg-name:"profile" description:"profile name(s) to apply, in order"`
	} `positional-args:"yes"`
}

// Execute is the go-flags entry point for `dfm apply`.
func (c *ApplyCmd) Execute(_ []string) error {
	baseAbs, err := filepath.Abs(c.globals.BaseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}

	paths, err := resolveProfilePaths(baseAbs, c.globals.ConfigPath, c.Args.Profiles)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	r := &stdoutReporter{}
	eng := engine.New(baseAbs, r)
	eng.DryRun = c.DryRun
	totals := engine.Tally{}

	for _, p := range paths {
		cfg, err := config.Load(p)
		if err != nil {
			return err
		}

		if c.DryRun {
			r.Info("would apply %s", p)
		} else {
			r.Info("applying %s", p)
		}

		tally, err := eng.Apply(c.Context(), cfg)
		totals = add(totals, tally)
		if err != nil {
			return fmt.Errorf("apply %s: %w", p, err)
		}
	}

	if !c.DryRun {
		if err := state.Save(&state.State{
			LastApplied: c.Args.Profiles,
			AppliedAt:   time.Now().UTC(),
			Links:       collectLinks(eng.Actions),
		}); err != nil {
			r.Warn("state save: %v", err)
		}
	}

	prefix := ""
	if c.DryRun {
		prefix = "[dry-run] "
	}

	r.Info("%sdone: %d links ok, %d created, %d relinked, %d backed up, %d shell ran (%d failed), %d cleaned, %d dirs created",
		prefix, totals.LinksOK, totals.LinksCreated, totals.LinksRelinked, totals.LinksBackedUp,
		totals.ShellRun, totals.ShellFailed, totals.Cleaned, totals.Created)

	return nil
}

// collectLinks picks out the symlinks the engine created or confirmed, so
// `dfm doctor` can later verify them. Skipped or backed-up actions aren't
// recorded — doctor should only ask about links dfm still owns.
func collectLinks(actions []engine.Action) []state.Link {
	var out []state.Link

	for _, a := range actions {
		switch a.Kind {
		case engine.ActionLinkCreate, engine.ActionLinkRelink, engine.ActionLinkExists:
			out = append(out, state.Link{Target: a.From, Source: a.To})
		}
	}
	return out
}

// add sums two Tally values field-by-field so totals can span profiles.
func add(a, b engine.Tally) engine.Tally {
	return engine.Tally{
		LinksOK:       a.LinksOK + b.LinksOK,
		LinksCreated:  a.LinksCreated + b.LinksCreated,
		LinksRelinked: a.LinksRelinked + b.LinksRelinked,
		LinksBackedUp: a.LinksBackedUp + b.LinksBackedUp,
		ShellRun:      a.ShellRun + b.ShellRun,
		ShellFailed:   a.ShellFailed + b.ShellFailed,
		Cleaned:       a.Cleaned + b.Cleaned,
		Created:       a.Created + b.Created,
	}
}

// stdoutReporter prints engine messages to the console. Actions go to
// stdout (they describe user-visible changes); warnings and info go to
// stderr so they don't pollute scripted pipelines.
type stdoutReporter struct{}

func (stdoutReporter) Action(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "• "+format+"\n", args...)
}
func (stdoutReporter) Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  "+format+"\n", args...)
}
func (stdoutReporter) Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "! "+format+"\n", args...)
}
