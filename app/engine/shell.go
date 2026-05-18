package engine

import (
	"context"
	"os"
	"os/exec"

	"log"

	"github.com/bitcldr/dfm/app/config"
)

// runShell executes a `shell:` directive. Each command runs under /bin/sh,
// with cwd = BaseDir. A single failing command does not abort the directive —
// a failing command is recorded but does not abort the directive.
func (e *Engine) runShell(ctx context.Context, s *config.Shell, tally *Tally) error {
	for _, item := range s.Entries {
		opts := mergeShellOpts(e.Defaults.Shell, item.Options)
		quiet := boolOr(opts.Quiet, false)
		if quiet && item.Description != "" {
			log.Printf("[INFO] %s", item.Description)
		} else if !quiet {
			if item.Description != "" {
				log.Printf("[INFO] %s [%s]", item.Description, item.Command)
			} else {
				log.Printf("[INFO] %s", item.Command)
			}
		}

		log.Printf("[DEBUG] shell cmd=%q dir=%s dry_run=%v", item.Command, e.BaseDir, e.DryRun)
		e.record(ActionShellRun, item.Command, item.Description)
		if e.DryRun {
			continue
		}

		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", item.Command) //nolint:gosec // shell execution is the whole point
		cmd.Dir = e.BaseDir
		if boolOr(opts.Stdin, false) {
			cmd.Stdin = os.Stdin
		}
		if !quiet && boolOr(opts.Stdout, false) {
			cmd.Stdout = os.Stdout
		}
		if !quiet && boolOr(opts.Stderr, false) {
			cmd.Stderr = os.Stderr
		}
		tally.ShellRun++
		if err := cmd.Run(); err != nil {
			tally.ShellFailed++
			log.Printf("[WARN] command failed [%s]: %v", item.Command, err)
		}
	}
	return nil
}
