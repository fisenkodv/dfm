package engine

import (
	"errors"
	"log"
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
			log.Printf("[DEBUG] create stat path=%s exists=true", path)
			e.io().PathExists(path)
			e.record(ActionCreateExists, path, "")
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Printf("[WARN] stat %s: %v", path, err)
			continue
		}

		if !e.DryRun {
			if err := os.MkdirAll(path, mode); err != nil {
				log.Printf("[WARN] create %s: %v", path, err)
				continue
			}

			if err := os.Chmod(path, mode); err != nil {
				log.Printf("[DEBUG] chmod failed path=%s err=%v", path, err)
			} else {
				log.Printf("[DEBUG] chmod path=%s mode=%04o", path, mode)
			}
		}

		e.io().Created(path)
		e.record(ActionCreateDir, path, "")
		tally.Created++
	}

	return nil
}
