package workflow

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"skill-audit/internal/audit"
)

// Exact lexical containment remains the fast path. A case-folded spelling is
// only a candidate: filesystem identity must confirm it. This avoids merging
// distinct case-sensitive directories on Linux (or case-sensitive macOS).
func sameDirectoryPath(a, b string) bool {
	if b == "" {
		return false
	}
	var err error
	a, err = filepath.Abs(a)
	if err != nil {
		return false
	}
	b, err = filepath.Abs(b)
	if err != nil {
		return false
	}
	if a == b {
		return true
	}
	if !strings.EqualFold(a, b) {
		return false
	}
	ai, ae := os.Lstat(a)
	bi, be := os.Lstat(b)
	return ae == nil && be == nil && ai.IsDir() && bi.IsDir() && os.SameFile(ai, bi)
}

// Delegate selected subtrees even when an overlapping root uses different case.
// Only the small explicit root set is compared; no per-entry visited map grows.
// Selected link paths remain invalid rather than becoming aliases.
func selectedDirectory(p string, d fs.DirEntry, roots map[string]bool) bool {
	if roots[p] {
		return true
	}
	for root := range roots {
		if !strings.EqualFold(p, root) {
			continue
		}
		if audit.NoSymlinks(root) != nil {
			continue
		}
		other, err := os.Lstat(root)
		if err != nil || !other.IsDir() {
			continue
		}
		here, err := d.Info()
		if err == nil && os.SameFile(here, other) {
			return true
		}
	}
	return false
}
