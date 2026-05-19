package engine

import (
	"context"
	"log"
	"os/exec"

	"github.com/bitcldr/dfm/app/config"
)

// runShell executes a `shell:` directive. Each command runs under /bin/sh,
// with cwd = BaseDir. A single failing command does not abort the directive —
// a failing command is recorded but does not abort the directive.
func (e *Engine) runShell(ctx context.Context, s *config.Shell, tally *Tally) error {
	for _, item := range s.Entries {
		opts := mergeShellOpts(e.Defaults.Shell, item.Options)
		quiet := boolOr(opts.Quiet, false)
		e.io().ShellCmd(item.Description, item.Command, quiet)

		log.Printf("[DEBUG] shell cmd=%q dir=%s dry_run=%v", item.Command, e.BaseDir, e.DryRun)
		e.record(ActionShellRun, item.Command, item.Description)
		if e.DryRun {
			continue
		}

		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", item.Command) //nolint:gosec // shell execution is the whole point
		cmd.Dir = e.BaseDir
		io := e.io()
		if boolOr(opts.Stdin, false) {
			cmd.Stdin = io.In
		}
		if !quiet && boolOr(opts.Stdout, false) {
			cmd.Stdout = io.Out
		}
		if !quiet && boolOr(opts.Stderr, false) {
			cmd.Stderr = io.ErrOut
		}
		tally.ShellRun++
		if err := cmd.Run(); err != nil {
			tally.ShellFailed++
			log.Printf("[WARN] command failed [%s]: %v", item.Command, err)
		}
	}
	return nil
}
