package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fisenkodv/dfm/app/config"
)

// runLink executes a `link:` directive.
func (e *Engine) runLink(l *config.Link, tally *Tally) error {
	for _, entry := range l.Entries {
		if err := e.linkOne(entry, tally); err != nil {
			// Log and continue on recoverable link errors.
			e.Reporter.Warn("link %s: %v", entry.Target, err)
		}
	}
	return nil
}

// linkOne handles a single target→source pair. Non-glob path; glob is
// deferred to a follow-up once needed (the user's actual profiles don't use
// it, so v1 ships without it).
func (e *Engine) linkOne(entry config.LinkEntry, tally *Tally) error {
	opts := mergeLinkOpts(e.Defaults.Link, entry.Options)

	// Source path comes from opts.Path; if absent, use the target's basename
	// stripped of a leading dot (e.g. "~/.vim" → "vim").
	source := strOr(opts.Path, defaultSource(entry.Target))
	sourceAbs := e.resolveBase(source)

	linkPath, err := filepath.Abs(expand(entry.Target))
	if err != nil {
		return fmt.Errorf("abs: %w", err)
	}

	if boolOr(opts.Glob, false) && hasGlobChars(source) {
		return fmt.Errorf("glob: not yet supported in v1")
	}

	// Validate source exists unless ignore-missing is set.
	if !boolOr(opts.IgnoreMissing, false) {
		if _, err := os.Stat(sourceAbs); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("nonexistent source %s", sourceAbs)
			}
			return fmt.Errorf("stat source: %w", err)
		}
	}

	// Optionally create the parent directory of the link.
	if boolOr(opts.Create, false) && !e.DryRun {
		if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}
	}

	// Compute the desired link target string. If relative=true, store a
	// relative path from the link's directory rather than the absolute
	// source. Dotbot does the same.
	desired := sourceAbs
	if boolOr(opts.Relative, false) {
		rel, err := filepath.Rel(filepath.Dir(linkPath), sourceAbs)
		if err != nil {
			return fmt.Errorf("relative: %w", err)
		}
		desired = rel
	}

	linkType := strOr(opts.Type, "symlink")
	if linkType != "symlink" {
		return fmt.Errorf("link type %q not supported in v1", linkType)
	}

	// Inspect existing path at linkPath.
	existing, existingKind, err := lstatKind(linkPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// No entry — create.
		return e.createSymlink(linkPath, desired, tally)
	case err != nil:
		return fmt.Errorf("lstat %s: %w", linkPath, err)
	}

	switch existingKind {
	case kindSymlink:
		current, err := os.Readlink(linkPath)
		if err != nil {
			return fmt.Errorf("readlink: %w", err)
		}
		if current == desired {
			e.Reporter.Info("link ok %s -> %s", entry.Target, desired)
			e.record(ActionLinkExists, linkPath, desired)
			tally.LinksOK++
			return nil
		}
		if boolOr(opts.Relink, false) || boolOr(opts.Force, false) {
			if !e.DryRun {
				if err := os.Remove(linkPath); err != nil {
					return fmt.Errorf("remove stale link: %w", err)
				}
				if err := e.writeSymlink(linkPath, desired); err != nil {
					return err
				}
			}
			e.Reporter.Action("relinked %s -> %s", linkPath, desired)
			e.record(ActionLinkRelink, linkPath, desired)
			tally.LinksRelinked++
			return nil
		}
		e.record(ActionLinkSkip, linkPath, desired)
		return fmt.Errorf("incorrect link %s -> %s (want %s); enable relink to replace",
			entry.Target, current, desired)

	case kindRegular, kindDir:
		// Non-symlink exists at target: back up then replace.
		// Backups are reversible, making apply idempotent.
		if err := e.ensureBackup(linkPath, existing.IsDir()); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		tally.LinksBackedUp++
		if err := e.createSymlink(linkPath, desired, tally); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unexpected file kind at %s", linkPath)
	}
}

// createSymlink creates a symlink (or records it in dry-run mode) and
// updates the tally. Callers must have already resolved conflicts.
func (e *Engine) createSymlink(linkPath, target string, tally *Tally) error {
	if !e.DryRun {
		if err := e.writeSymlink(linkPath, target); err != nil {
			return err
		}
	}
	e.Reporter.Action("linked %s -> %s", linkPath, target)
	e.record(ActionLinkCreate, linkPath, target)
	tally.LinksCreated++
	return nil
}

// writeSymlink is the raw os.Symlink call, isolated so it's the only place
// that mutates when DryRun is off.
func (e *Engine) writeSymlink(linkPath, target string) error {
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}
	return nil
}

// defaultSource strips a single leading "." from the target's basename.
// "~/.vim" -> "vim".
func defaultSource(target string) string {
	base := filepath.Base(target)
	if len(base) > 1 && base[0] == '.' {
		return base[1:]
	}
	return base
}

type fileKind int

const (
	kindUnknown fileKind = iota
	kindRegular
	kindDir
	kindSymlink
)

func lstatKind(path string) (os.FileInfo, fileKind, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, kindUnknown, err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return fi, kindSymlink, nil
	case fi.IsDir():
		return fi, kindDir, nil
	default:
		return fi, kindRegular, nil
	}
}

func hasGlobChars(s string) bool {
	for _, c := range s {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}
