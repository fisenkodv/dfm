package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/fisenkodv/dfm/app/state"
)

// DoctorCmd verifies that previously applied symlinks still resolve to the
// expected source files and reports drift.
type DoctorCmd struct {
	base
}

// Execute is the go-flags entry point for `dfm doctor`.
//
// Exits non-zero (via returned error) when any link is missing, not a
// symlink, or pointing at the wrong source. A clean run prints a summary
// and returns nil. This makes `dfm doctor` scriptable in CI.
func (c *DoctorCmd) Execute(_ []string) error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	if s == nil {
		fmt.Println("no applied state found — run `dfm apply` first")
		return nil
	}

	var problems []string
	ok := 0
	for _, l := range s.Links {
		dest, err := os.Readlink(l.Target)
		if errors.Is(err, os.ErrNotExist) {
			problems = append(problems, fmt.Sprintf("missing: %s", l.Target))
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("not a symlink: %s (%v)", l.Target, err))
			continue
		}
		if dest != l.Source {
			problems = append(problems, fmt.Sprintf("drifted: %s -> %s (want %s)", l.Target, dest, l.Source))
			continue
		}
		ok++
	}

	fmt.Printf("checked %d link(s): %d ok, %d problem(s)\n", len(s.Links), ok, len(problems))
	for _, p := range problems {
		fmt.Printf("  ! %s\n", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d link(s) need attention", len(problems))
	}
	return nil
}
