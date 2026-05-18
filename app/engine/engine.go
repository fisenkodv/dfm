// Package engine executes a parsed Config against the filesystem.
//
// Each directive's executor runs in the order it appears in the config.
// Executors are serial on purpose: link creation can depend on shell output
// from an earlier step.
package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitcldr/dfm/app/cond"
	"github.com/bitcldr/dfm/app/config"
)

// Tally counts what happened across one Apply call. Fields are updated
// directly by directive executors.
type Tally struct {
	LinksOK       int
	LinksCreated  int
	LinksRelinked int
	LinksBackedUp int
	ShellRun      int
	ShellFailed   int
	Cleaned       int
	Created       int
}

// Engine applies a parsed Config to the filesystem. BaseDir is the directory
// containing the profile (usually the dotfiles repo root); all source paths
// in link directives are resolved relative to it.
//
// Set DryRun to true to record intended actions without touching the
// filesystem. In that mode, directive executors still inspect the FS to
// decide what *would* happen but skip every mutation.
type Engine struct {
	BaseDir  string
	DryRun   bool
	Actions   []Action       // recorded in both real and dry-run modes
	Defaults  mergedDefaults // accumulated across `defaults:` directives
	Backup    backupWriter   // lazily created on first conflict
	backupTag string         // shared timestamp tag for all backups in this Apply
	Cond      cond.Context   // set by New; overridable for tests
}

// New builds an Engine for a given base directory. BaseDir must be absolute.
func New(baseDir string) *Engine {
	return &Engine{BaseDir: baseDir, Cond: cond.DefaultContext()}
}

// record appends an Action. Pulled into one helper so callers don't repeat
// the DryRun-threading boilerplate.
func (e *Engine) record(kind ActionKind, from, to string) {
	e.Actions = append(e.Actions, Action{Kind: kind, From: from, To: to, DryRun: e.DryRun})
}

// Apply runs every directive in cfg against the filesystem, returning a
// Tally and the first fatal error encountered. Non-fatal issues (a single
// shell command failing, a single link being skipped) accumulate in the
// Tally without aborting.
func (e *Engine) Apply(ctx context.Context, cfg *config.Config) (Tally, error) {
	var tally Tally

	for _, d := range cfg.Directives {
		if err := ctx.Err(); err != nil {
			return tally, err
		}

		log.Printf("[DEBUG] directive kind=%s line=%d", d.Kind, d.Line)

		if d.When != "" {
			ok, err := cond.Eval(d.When, e.Cond)
			if err != nil {
				return tally, fmt.Errorf("when (line %d): %w", d.Line, err)
			}
			log.Printf("[DEBUG] when=%q result=%v", d.When, ok)
			if !ok {
				continue
			}
		}

		switch d.Kind {
		case config.KindDefaults:
			e.Defaults.merge(d.Defaults)
		case config.KindLink:
			if err := e.runLink(d.Link, &tally); err != nil {
				return tally, err
			}
		case config.KindShell:
			if err := e.runShell(ctx, d.Shell, &tally); err != nil {
				return tally, err
			}
		case config.KindClean:
			if err := e.runClean(d.Clean, &tally); err != nil {
				return tally, err
			}
		case config.KindCreate:
			if err := e.runCreate(d.Create, &tally); err != nil {
				return tally, err
			}
		default:
			return tally, fmt.Errorf("engine: unknown directive %q at line %d", d.Kind, d.Line)
		}
	}

	return tally, nil
}

// mergedDefaults carries the running defaults as the engine iterates.
// Later `defaults:` directives override earlier values key-by-key.
type mergedDefaults struct {
	Link  config.LinkOptions
	Shell config.ShellOptions
	Clean config.CleanOptions
}

func (m *mergedDefaults) merge(d *config.Defaults) {
	if d == nil {
		return
	}

	if d.Link != nil {
		m.Link = mergeLinkOpts(m.Link, *d.Link)
	}

	if d.Shell != nil {
		m.Shell = mergeShellOpts(m.Shell, *d.Shell)
	}

	if d.Clean != nil {
		m.Clean = mergeCleanOpts(m.Clean, *d.Clean)
	}
}

// mergeLinkOpts returns a copy of base with non-nil fields from overlay
// overriding. Slice fields replace wholesale when overlay is non-empty.
func mergeLinkOpts(base, overlay config.LinkOptions) config.LinkOptions {
	if overlay.Path != nil {
		base.Path = overlay.Path
	}
	if overlay.Create != nil {
		base.Create = overlay.Create
	}
	if overlay.Relink != nil {
		base.Relink = overlay.Relink
	}
	if overlay.Force != nil {
		base.Force = overlay.Force
	}
	if overlay.Relative != nil {
		base.Relative = overlay.Relative
	}
	if overlay.Glob != nil {
		base.Glob = overlay.Glob
	}
	if overlay.IgnoreMissing != nil {
		base.IgnoreMissing = overlay.IgnoreMissing
	}
	if overlay.Backup != nil {
		base.Backup = overlay.Backup
	}
	if overlay.Type != nil {
		base.Type = overlay.Type
	}
	if overlay.Canonicalize != nil {
		base.Canonicalize = overlay.Canonicalize
	}
	if overlay.Prefix != nil {
		base.Prefix = overlay.Prefix
	}
	if len(overlay.Exclude) > 0 {
		base.Exclude = overlay.Exclude
	}

	return base
}

func mergeShellOpts(base, overlay config.ShellOptions) config.ShellOptions {
	if overlay.Stdin != nil {
		base.Stdin = overlay.Stdin
	}
	if overlay.Stdout != nil {
		base.Stdout = overlay.Stdout
	}
	if overlay.Stderr != nil {
		base.Stderr = overlay.Stderr
	}
	if overlay.Quiet != nil {
		base.Quiet = overlay.Quiet
	}

	return base
}

func mergeCleanOpts(base, overlay config.CleanOptions) config.CleanOptions {
	if overlay.Force != nil {
		base.Force = overlay.Force
	}
	if overlay.Recursive != nil {
		base.Recursive = overlay.Recursive
	}

	return base
}

// boolOr returns *p if non-nil, else fallback.
func boolOr(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}

	return *p
}

func strOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}

	return *p
}

// resolveBase joins baseDir with a relative source. baseDir is already
// absolute; source may be "config/nvim" or similar.
func (e *Engine) resolveBase(source string) string {
	if filepath.IsAbs(source) {
		return source
	}

	return filepath.Join(e.BaseDir, source)
}

// expand performs tilde and env expansion on a path.
func expand(path string) string {
	return os.ExpandEnv(expandHome(path))
}

// expandHome replaces a leading "~" with the current user's home directory.
// ~user syntax for other users is not supported.
func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}

	if len(path) > 1 && path[1] != '/' {
		// ~user/foo — rare; fall back to HOME of current user for safety.
		home, _ := os.UserHomeDir()
		slash := strings.Index(path, "/")
		if slash < 0 {
			return home
		}
		return home + path[slash:]
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	return home + path[1:]
}
