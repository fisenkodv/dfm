package engine

import (
	"errors"
	"os"

	"github.com/bitcldr/dfm/app/config"
)

// runCreate mkdirs the listed paths. Idempotent: existing paths are left
// alone (no mode update).
func (e *Engine) runCreate(c *config.Create, tally *Tally) error {
	for _, entry := range c.Entries {
		path := expand(entry.Path)
		mode := os.FileMode(0o777)
		if entry.Mode != nil {
			mode = os.FileMode(*entry.Mode)
		}
		if _, err := os.Stat(path); err == nil {
			e.Reporter.Info("path exists %s", path)
			e.record(ActionCreateExists, path, "")
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			e.Reporter.Warn("stat %s: %v", path, err)
			continue
		}
		if !e.DryRun {
			if err := os.MkdirAll(path, mode); err != nil {
				e.Reporter.Warn("create %s: %v", path, err)
				continue
			}
			if err := os.Chmod(path, mode); err != nil {
				// Non-fatal; best-effort chmod after mkdir.
				slogDebug("chmod after create", "path", path, "err", err)
			}
		}
		e.Reporter.Action("created %s", path)
		e.record(ActionCreateDir, path, "")
		tally.Created++
	}
	return nil
}
