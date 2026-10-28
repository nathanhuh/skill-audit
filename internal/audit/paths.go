package audit

import (
	"path/filepath"
	"strings"
)

// Within tests lexical containment, including equality, using host path rules.
// It does not resolve links or filesystem aliases; callers must check those separately.
func Within(path, root string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, absolute)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
