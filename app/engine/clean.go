package engine

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitcldr/dfm/app/config"
)

// isInsideAny reports whether path sits inside any of the candidate prefixes.
// Each prefix is compared against the cleaned path with a trailing separator
// so "/foo/bar" isn't treated as inside "/foo/ba".
func isInsideAny(path string, prefixes []string) bool {
	p := filepath.Clean(path) + string(filepath.Separator)
	for _, pre := range prefixes {
		if strings.HasPrefix(p, filepath.Clean(pre)+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// runClean removes dead symlinks in the target directories. A dead link is
// removed only when it points into BaseDir (so we don't touch unrelated
// links) — unless force=true.
func (e *Engine) runClean(c *config.Clean, tally *Tally) error {
	// Collect every prefix that should be treated as "inside base dir".
	// On macOS, t.TempDir() produces /var/folders/... but EvalSymlinks
	// resolves to /private/var/folders/... — we must accept both.
	baseCandidates := []string{filepath.Clean(e.BaseDir)}
	if resolved, err := filepath.EvalSymlinks(e.BaseDir); err == nil {
		baseCandidates = append(baseCandidates, filepath.Clean(resolved))
	}

	for _, entry := range c.Entries {
		opts := mergeCleanOpts(e.Defaults.Clean, entry.Options)
		target := expand(entry.Target)

		if err := e.cleanDir(target, baseCandidates, boolOr(opts.Force, false), boolOr(opts.Recursive, false), tally); err != nil {
			log.Printf("[WARN] clean %s: %v", entry.Target, err)
		}
	}

	return nil
}

func (e *Engine) cleanDir(dir string, baseCandidates []string, force, recursive bool, tally *Tally) error {
	fi, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !fi.IsDir()) {
		return nil
	}
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, de := range entries {
		path := filepath.Join(dir, de.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		if recursive && info.IsDir() {
			if err := e.cleanDir(path, baseCandidates, force, recursive, tally); err != nil {
				log.Printf("[WARN] clean %s: %v", path, err)
			}
			continue
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		// Symlink — check if it's broken (target doesn't exist).
		if _, err := os.Stat(path); err == nil {
			log.Printf("[DEBUG] clean skip %s: still resolves", path)
			continue
		}

		points, err := os.Readlink(path)
		if err != nil {
			continue
		}

		if !filepath.IsAbs(points) {
			points = filepath.Join(filepath.Dir(path), points)
		}

		if !force && !isInsideAny(points, baseCandidates) {
			log.Printf("[DEBUG] clean skip %s: points outside base dir (points=%s)", path, points)
			continue // links outside base dir are left alone
		}
		log.Printf("[DEBUG] clean remove %s -> %s force=%v", path, points, force)

		if !e.DryRun {
			if err := os.Remove(path); err != nil {
				log.Printf("[WARN] remove %s: %v", path, err)
				continue
			}
		}

		e.io().RemovedDeadLink(path, points)
		e.record(ActionCleanRemove, path, points)
		tally.Cleaned++
	}

	return nil
}
