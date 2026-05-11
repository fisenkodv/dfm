package engine

import (
	"context"
	"os"
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
		if quiet && item.Description != "" {
			e.Reporter.Info("%s", item.Description)
		} else if !quiet {
			if item.Description != "" {
				e.Reporter.Action("%s [%s]", item.Description, item.Command)
			} else {
				e.Reporter.Action("%s", item.Command)
			}
		}

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
		if err := cmd.Run(); err != nil {
			tally.ShellFailed++
			e.Reporter.Warn("command failed [%s]: %v", item.Command, err)
			continue
		}
		tally.ShellRun++
	}
	return nil
}
