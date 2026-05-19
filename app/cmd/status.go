package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/bitcldr/dfm/app/state"
)

// StatusCmd prints the profiles that were most recently applied on this
// machine, along with the apply timestamp.
type StatusCmd struct {
	base
}

// Execute is the go-flags entry point for `dfm status`.
func (c *StatusCmd) Execute(_ []string) error {
	ios := c.IO()

	s, err := state.Load()
	if err != nil {
		return err
	}

	if s == nil {
		ios.StatusEmpty("no profiles have been applied on this machine yet")
		return nil
	}

	p, _ := state.Path()
	ios.StatusLine("State file:  ", p)
	ios.StatusLine("Last applied:", strings.Join(s.LastApplied, " "))
	ios.StatusLineWithMeta(
		"Applied at:  ",
		s.AppliedAt.Local().Format(time.RFC3339),
		"("+humanSince(s.AppliedAt)+" ago)",
	)
	ios.StatusLine("Links:       ", fmt.Sprintf("%d", len(s.Links)))

	return nil
}

// humanSince returns a coarse human-readable duration like "3h" or "2d".
// Deliberately coarse — status is glanceable, not a stopwatch.
func humanSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
