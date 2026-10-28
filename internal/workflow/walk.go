package workflow

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"skill-audit/internal/audit"
)

// Priority affects which evidence survives a bounded scan, not its completeness.
// Lexical ordering within each group keeps results reproducible. No paths outside
// the selected roots are introduced; home is only a classification/ordering hint.
func directoryPriority(p, home string) int {
	if home != "" && audit.Within(home, p) {
		return 0
	}
	if agentDirectory(filepath.Base(p)) {
		return 1
	}
	switch filepath.Base(p) {
	case "Users", "home", "root":
		return 2
	}
	return 3
}

// Like WalkDir, call visit before opening a directory and again on read errors.
// The caller enforces depth/entry limits, symlink skipping and cancellation.
// Reuse DirEntry metadata: never follow links or stat every child just to sort.
func walkInventory(p string, d fs.DirEntry, home string, visit fs.WalkDirFunc) error {
	if err := visit(p, d, nil); err != nil || !d.IsDir() {
		return err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		if err = visit(p, d, err); err != nil {
			return err
		}
	}
	// ReadDir returns lexical order; stable sorting preserves it within a group.
	priority := func(d fs.DirEntry) int {
		if !d.IsDir() {
			return 3
		}
		return directoryPriority(filepath.Join(p, d.Name()), home)
	}
	sort.SliceStable(entries, func(i, j int) bool { return priority(entries[i]) < priority(entries[j]) })
	for _, child := range entries {
		err := walkInventory(filepath.Join(p, child.Name()), child, home, visit)
		if err == fs.SkipDir {
			if child.IsDir() {
				continue
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}
