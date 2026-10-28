package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skill-audit/internal/audit"
)

type fileOption struct {
	flag, path string
	writes     bool
}

// Validate file roles before work and again before writes. Atomic replacement
// protects a write's completeness; it cannot protect a wrongly chosen target.
// Symlinks are unsupported, and case-only aliases are rejected conservatively
// even on case-sensitive filesystems. This is not a hostile-race guarantee.
func validateFilePaths(options ...fileOption) error {
	type resolved struct {
		fileOption
		absolute string
		info     os.FileInfo
	}
	paths := []resolved{}
	for _, option := range options {
		if option.path == "" {
			continue
		}
		absolute, err := filepath.Abs(option.path)
		if err != nil {
			return err
		}
		if err = audit.NoSymlinks(absolute); err != nil {
			return fmt.Errorf("%s: %w", option.flag, err)
		}
		info, err := os.Stat(absolute)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", option.flag, err)
		}
		paths = append(paths, resolved{option, absolute, info})
	}
	for i, a := range paths {
		for _, b := range paths[i+1:] {
			if !a.writes && !b.writes {
				continue
			}
			if strings.EqualFold(a.absolute, b.absolute) || (a.info != nil && b.info != nil && os.SameFile(a.info, b.info)) {
				return fmt.Errorf("%s and %s must refer to different files", a.flag, b.flag)
			}
		}
	}
	return nil
}
