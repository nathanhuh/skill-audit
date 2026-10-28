package workflow

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"skill-audit/internal/audit"
)

const InventorySchema = "skill-audit/inventory/v2"

type Package struct {
	Name     string       `json:"name"`
	Scope    string       `json:"scope"`
	Path     string       `json:"path"`
	Agent    string       `json:"agent"`
	Digest   string       `json:"digest"`
	Complete bool         `json:"complete"`
	Files    []audit.File `json:"files"`
}
type Inventory struct {
	Schema      string             `json:"schema"`
	Roots       []string           `json:"roots"`
	Packages    []Package          `json:"packages"`
	Diagnostics []audit.Diagnostic `json:"diagnostics"`
	Entries     int                `json:"entries"`
}
type Limits struct{ Entries, Packages, Depth int }

func DefaultLimits() Limits { return Limits{500000, 2000, 64} }
func DeviceRoots() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	roots := []string{}
	for drive := 'A'; drive <= 'Z'; drive++ {
		p := fmt.Sprintf("%c:\\", drive)
		if i, e := os.Stat(p); e == nil && i.IsDir() {
			roots = append(roots, p)
		}
	}
	return roots
}
func Discover(ctx context.Context, roots []string, home string, limits Limits) Inventory {
	out := Inventory{Schema: InventorySchema, Roots: []string{}, Packages: []Package{}, Diagnostics: []audit.Diagnostic{}}
	issue := func(p, code, msg string) {
		out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: p, Code: code, Message: msg, Incomplete: code != "excluded_subtree"})
	}
	stopped := ""
	rootset := map[string]bool{}
	for _, root := range roots {
		p, e := filepath.Abs(root)
		if e != nil {
			issue(root, "root_error", e.Error())
			continue
		}
		if !rootset[p] {
			rootset[p] = true
			out.Roots = append(out.Roots, p)
		}
	}
	sort.Strings(out.Roots)
	walkRoots := append([]string{}, out.Roots...)
	sort.SliceStable(walkRoots, func(i, j int) bool {
		pi, pj := rootPriority(walkRoots[i], home), rootPriority(walkRoots[j], home)
		if pi != pj {
			return pi < pj
		}
		return rootDepth(walkRoots[i]) > rootDepth(walkRoots[j])
	})
	interrupted := func() string {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "inventory_timeout"
		}
		return "inventory_canceled"
	}
	pending := func(roots []string, code, reason string) {
		for _, root := range roots {
			issue(root, code, "root not started: "+reason)
		}
	}
	for index, root := range walkRoots {
		if ctx.Err() != nil {
			pending(walkRoots[index:], interrupted(), ctx.Err().Error())
			break
		}
		if e := audit.NoSymlinks(root); e != nil {
			issue(root, "symlink", e.Error())
			continue
		}
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() {
			issue(root, "root_error", "root unavailable or not a directory")
			continue
		}
		e := walkInventory(root, fs.FileInfoToDirEntry(info), home, func(p string, d fs.DirEntry, e error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if e != nil {
				issue(p, "read_error", e.Error())
				return nil
			}
			// A selected descendant gets its own traversal, regardless of which
			// ancestor has priority. Pending-root diagnostics cover interruptions.
			if d.IsDir() && p != root && rootset[p] {
				return fs.SkipDir
			}

			out.Entries++
			if out.Entries > limits.Entries {
				stopped = "entry_limit"
				issue(p, "entry_limit", "remaining inventory traversal stopped")
				return fs.SkipAll
			}
			rel, _ := filepath.Rel(root, p)
			if rel != "." && strings.Count(rel, string(filepath.Separator)) >= limits.Depth && d.IsDir() {
				issue(p, "depth_limit", "subtree skipped")
				return fs.SkipDir
			}
			if excluded(p, d.Name(), root) {
				code := "excluded_subtree"
				if p == root {
					code = "excluded_root"
				}
				issue(p, code, "outside device-search policy; entry not inspected")
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				issue(p, "policy_symlink", "link not followed; target skill coverage is unknown")
				return nil
			}
			if d.IsDir() {
				return nil
			}

			if d.Name() != "SKILL.md" {
				return nil
			}
			if !d.Type().IsRegular() {
				issue(p, "special_manifest", "not a regular file")
				return nil
			}
			if len(out.Packages) >= limits.Packages {
				stopped = "package_limit"
				issue(p, "package_limit", "remaining inventory traversal stopped")
				return fs.SkipAll
			}
			dir := filepath.Dir(p)
			scope, agent := classify(dir, home)
			s := audit.Inspect(ctx, scope, dir)
			out.Packages = append(out.Packages, Package{s.Name, scope, dir, agent, s.Digest, s.Complete, s.Files})
			out.Diagnostics = append(out.Diagnostics, s.Diagnostics...)
			return nil
		})
		if ctx.Err() != nil {
			issue(root, interrupted(), "traversal interrupted: "+ctx.Err().Error())
			pending(walkRoots[index+1:], interrupted(), ctx.Err().Error())
			break
		}
		if e != nil && e != fs.SkipAll && e != fs.SkipDir {
			issue(root, "walk_error", e.Error())
		}
		if stopped != "" {
			pending(walkRoots[index+1:], stopped, "inventory budget exhausted")
			break
		}
	}
	sort.Slice(out.Packages, func(i, j int) bool { return out.Packages[i].Path < out.Packages[j].Path })
	return out
}
func excluded(p, name, root string) bool {
	return excludedOnOS(p, name, root, runtime.GOOS)
}

func excludedOnOS(p, name, root, goos string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	virtual := []string{"/proc", "/sys", "/dev"}
	// The unified macOS tree exposes Data through firmlinks. Walking the
	// alternate mount as well duplicates identities and consumes scan budgets.
	// Keep this an explicit coverage exclusion, not a completeness claim.
	if goos == "darwin" {
		virtual = append(virtual, "/System/Volumes/Data")
	}
	for _, v := range virtual {
		if clean == v || strings.HasPrefix(clean, v+"/") {
			return true
		}
	}
	if p == root {
		return false
	}
	if audit.ExcludedMetadata(name) {
		return true
	}
	switch name {
	case "node_modules", "vendor", ".venv", "venv", "__pycache__":
		return true
	}
	return false
}
func agentDirectory(name string) bool {
	switch name {
	case ".claude", ".codex", ".agents", ".cursor", ".gemini", ".github", ".opencode":
		return true
	}
	return false
}

func classify(p, home string) (string, string) {
	// Prefer the nearest agent marker, so a project nested in an agent worktree
	// does not inherit global scope from the outer home configuration directory.
	for dir := filepath.Clean(p); ; dir = filepath.Dir(dir) {
		agent := filepath.Base(dir)
		if agentDirectory(agent) {
			parent := filepath.Dir(dir)
			rel, _ := filepath.Rel(dir, p)
			first := strings.Split(filepath.ToSlash(rel), "/")[0]
			installation := first != "worktrees" && first != "projects"
			if agent != ".github" && installation && ((home != "" && audit.Within(parent, home) && audit.Within(home, parent)) || conventionalHome(parent)) {
				return "global", agent
			}
			return "project", agent
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "machine", "unknown"
}
func conventionalHome(p string) bool {
	slash := filepath.ToSlash(p)
	parts := strings.Split(strings.Trim(slash, "/"), "/")
	if strings.HasPrefix(slash, "/") {
		return len(parts) == 1 && parts[0] == "root" || len(parts) == 2 && (parts[0] == "Users" || parts[0] == "home")
	}
	return len(parts) == 3 && strings.HasSuffix(parts[0], ":") && strings.EqualFold(parts[1], "Users")
}
