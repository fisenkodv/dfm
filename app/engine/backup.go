package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// backupWriter lazily owns the session backup directory
// (~/.dotfiles-backup/<rfc3339>/...). One writer per Engine per Apply call.
type backupWriter struct {
	root string // absolute path of the session backup dir
}

func (e *Engine) initBackupRoot() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	if e.backupTag == "" {
		e.backupTag = time.Now().UTC().Format("20060102T150405Z")
	}
	e.Backup.root = filepath.Join(home, ".dotfiles-backup", e.backupTag)
	if e.DryRun {
		return nil
	}
	return os.MkdirAll(e.Backup.root, 0o750)
}

func (e *Engine) ensureBackup(src string, isDir bool) error {
	if e.Backup.root == "" {
		if err := e.initBackupRoot(); err != nil {
			return err
		}
	}
	// Mirror the full absolute path under root so two backups of the same
	// basename don't collide.
	rel := src
	if filepath.IsAbs(rel) {
		rel = rel[1:] // drop leading "/"
	}
	dst := filepath.Join(e.Backup.root, rel)
	if !e.DryRun {
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
		}
	}
	e.Reporter.Action("backed up %s -> %s", src, dst)
	e.record(ActionLinkBackup, src, dst)
	_ = isDir // reserved for future split behavior (e.g. archive dirs)
	return nil
}
