package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListCmd enumerates profile files under BaseDir/profiles/.
type ListCmd struct {
	base
}

// Execute is the go-flags entry point for `dfm list`.
func (c *ListCmd) Execute(_ []string) error {
	baseAbs, err := filepath.Abs(c.globals.BaseDir)
	if err != nil {
		return err
	}
	dir := filepath.Join(baseAbs, "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list: read %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if n, ok := strings.CutSuffix(e.Name(), ".conf.yaml"); ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}
