package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitcldr/dfm/app/config"
	"github.com/bitcldr/dfm/app/engine"
)

// DiffCmd shows the set of filesystem changes that would result from
// applying the given profiles, without touching disk. Output groups actions
// by kind so it's scannable at a glance.
type DiffCmd struct {
	base
	Args struct {
		Profiles []string `positional-arg-name:"profile"`
	} `positional-args:"yes"`
}

// Execute is the go-flags entry point for `dfm diff`.
func (c *DiffCmd) Execute(_ []string) error {
	baseAbs, err := filepath.Abs(c.globals.BaseDir)
	if err != nil {
		return err
	}

	paths, err := resolveProfilePaths(baseAbs, c.globals.ConfigPath, c.Args.Profiles)
	if err != nil {
		return err
	}

	r := quietReporter{}
	eng := engine.New(baseAbs, r)
	eng.DryRun = true

	for _, p := range paths {
		cfg, err := config.Load(p)
		if err != nil {
			return err
		}

		if _, err := eng.Apply(c.Context(), cfg); err != nil {
			return fmt.Errorf("diff %s: %w", p, err)
		}
	}

	printDiff(os.Stdout, eng.Actions)
	return nil
}

// quietReporter discards engine chatter during diff — we render from the
// structured Actions list instead so output is stable and scriptable.
type quietReporter struct{}

func (quietReporter) Action(string, ...any) {}
func (quietReporter) Info(string, ...any)   {}
func (quietReporter) Warn(string, ...any)   {}

// printDiff groups actions by kind and prints each group with a header. A
// legend shows what prefixes mean. Empty groups are omitted.
func printDiff(w *os.File, actions []engine.Action) {
	groups := map[engine.ActionKind][]engine.Action{}
	order := []engine.ActionKind{
		engine.ActionLinkCreate,
		engine.ActionLinkRelink,
		engine.ActionLinkBackup,
		engine.ActionLinkSkip,
		engine.ActionLinkExists,
		engine.ActionCreateDir,
		engine.ActionCreateExists,
		engine.ActionCleanRemove,
		engine.ActionShellRun,
	}

	for _, a := range actions {
		groups[a.Kind] = append(groups[a.Kind], a)
	}

	empty := true
	for _, k := range order {
		list := groups[k]
		if len(list) == 0 {
			continue
		}

		empty = false
		fmt.Fprintf(w, "%s (%d)\n", headerFor(k), len(list))

		for _, a := range list {
			fmt.Fprintf(w, "  %s\n", formatAction(a))
		}
	}

	if empty {
		fmt.Fprintln(w, "no changes")
	}
}

func headerFor(k engine.ActionKind) string {
	switch k {
	case engine.ActionLinkCreate:
		return "+ links to create"
	case engine.ActionLinkRelink:
		return "~ links to relink"
	case engine.ActionLinkBackup:
		return "! non-symlink targets to back up"
	case engine.ActionLinkSkip:
		return "? links blocked by conflict (need relink/force)"
	case engine.ActionLinkExists:
		return "= links already correct"
	case engine.ActionCreateDir:
		return "+ directories to create"
	case engine.ActionCreateExists:
		return "= directories already present"
	case engine.ActionCleanRemove:
		return "- dead links to remove"
	case engine.ActionShellRun:
		return "$ shell commands to run"
	}

	return "? unknown"
}

func formatAction(a engine.Action) string {
	switch a.Kind {
	case engine.ActionShellRun:
		if a.To != "" {
			return fmt.Sprintf("%s  [%s]", a.To, a.From)
		}
		return a.From
	case engine.ActionCreateDir, engine.ActionCreateExists:
		return a.From
	default:
		if a.To == "" {
			return a.From
		}
		return fmt.Sprintf("%s -> %s", a.From, a.To)
	}
}

// resolveProfilePaths is shared by apply and diff. Kept here rather than in
// apply.go to avoid one command importing the other.
func resolveProfilePaths(baseAbs, configPath string, profiles []string) ([]string, error) {
	if configPath != "" {
		p, err := filepath.Abs(configPath)
		if err != nil {
			return nil, err
		}

		return []string{p}, nil
	}

	if len(profiles) == 0 {
		return nil, errors.New("at least one profile is required")
	}

	out := make([]string, 0, len(profiles))

	for _, name := range profiles {
		p := filepath.Join(baseAbs, "profiles", name+".conf.yaml")
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("profile %q not found at %s", name, p)
		}

		out = append(out, p)
	}

	return out, nil
}
