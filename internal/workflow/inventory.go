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

const InventorySchema = "skill-audit/inventory/v3"
const DiscoveryPolicy = "skill-audit/discovery/v3"

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
	Policy      string             `json:"policy"`
	Home        string             `json:"home,omitempty"`
	Limits      Limits             `json:"limits"`
	Exclusions  []audit.Exclusion  `json:"exclusions"`
	Schema      string             `json:"schema"`
	Roots       []string           `json:"roots"`
	Packages    []Package          `json:"packages"`
	Diagnostics []audit.Diagnostic `json:"diagnostics"`
	Entries     int                `json:"entries"`
}
type Limits audit.TraversalLimits

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
	out := Inventory{Policy: DiscoveryPolicy, Home: home, Limits: limits, Exclusions: []audit.Exclusion{}, Schema: InventorySchema, Roots: []string{}, Packages: []Package{}, Diagnostics: []audit.Diagnostic{}}
	issue := func(p, code, msg string) {
		out.Diagnostics = append(out.Diagnostics, audit.Diagnostic{Path: p, Code: code, Message: msg, Incomplete: code != "excluded_subtree"})
	}
	exclusions := map[string]*audit.Exclusion{}
	noteExclusion := func(p, rule string) {
		e := exclusions[rule]
		if e == nil {
			e = &audit.Exclusion{Rule: rule, Examples: []string{}}
			exclusions[rule] = e
		}
		e.Count++
		if len(e.Examples) < 3 {
			e.Examples = append(e.Examples, p)
		} else {
			e.OmittedExamples++
		}
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
	walkedRoots := []fs.FileInfo{}
	for index, root := range walkRoots {
		if ctx.Err() != nil {
			pending(walkRoots[index:], interrupted(), ctx.Err().Error())
			break
		}
		if rule := exclusionRuleOnOS(root, filepath.Base(root), root, runtime.GOOS); rule != "" {
			noteExclusion(root, rule)
			issue(root, "excluded_root", "selected root excluded by discovery policy; not inspected")
			continue
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
		duplicate := false
		for _, previous := range walkedRoots {
			if os.SameFile(previous, info) {
				duplicate = true
				break
			}
		}
		if duplicate {
			noteExclusion(root, "duplicate_root")
			continue
		}
		walkedRoots = append(walkedRoots, info)
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
			if d.IsDir() && p != root && selectedDirectory(p, d, rootset) {
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
			// Ordinary files cannot be excluded subtrees. Metadata files and
			// links still need policy classification before no-follow handling.
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 || audit.ExcludedMetadata(d.Name()) {
				if rule := exclusionRuleOnOS(p, d.Name(), root, runtime.GOOS); rule != "" {
					noteExclusion(p, rule)
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
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
	for rule, e := range exclusions {
		sort.Strings(e.Examples)
		out.Exclusions = append(out.Exclusions, *e)
		issue("policy:"+rule, "excluded_subtree", fmt.Sprintf("rule=%s count=%d examples=%q omitted_examples=%d", rule, e.Count, e.Examples, e.OmittedExamples))
	}
	sort.Slice(out.Exclusions, func(i, j int) bool { return out.Exclusions[i].Rule < out.Exclusions[j].Rule })
	// Map iteration must not change diagnostic ordering.
	sort.SliceStable(out.Diagnostics, func(i, j int) bool {
		a, b := out.Diagnostics[i], out.Diagnostics[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	sort.Slice(out.Packages, func(i, j int) bool { return out.Packages[i].Path < out.Packages[j].Path })
	return out
}
func excluded(p, name, root string) bool {
	return excludedOnOS(p, name, root, runtime.GOOS)
}

func excludedOnOS(p, name, root, goos string) bool {
	return exclusionRuleOnOS(p, name, root, goos) != ""
}

// System rules use the matched policy root, never a selected descendant path.
var systemExclusions = [...]string{"/proc", "/sys", "/dev"}
var macAliases = [...]string{"/etc", "/tmp", "/var"}

func exclusionRuleOnOS(p, name, root, goos string) string {
	clean := filepath.ToSlash(filepath.Clean(p))
	match := func(v string) bool { return clean == v || strings.HasPrefix(clean, v+"/") }
	for _, v := range systemExclusions {
		if match(v) {
			return "system/" + strings.TrimPrefix(v, "/")
		}
	}
	// The alternate Data tree remains a hard exclusion, including explicit roots.
	if goos == "darwin" && match("/System/Volumes/Data") {
		return "system/System/Volumes/Data"
	}
	// Explicit aliases must reach NoSymlinks, giving their actual rejection cause.
	if p == root {
		return ""
	}
	if goos == "darwin" {
		for _, v := range macAliases {
			if match(v) {
				return "system/" + strings.TrimPrefix(v, "/")
			}
		}
	}
	if audit.ExcludedMetadata(name) {
		return "metadata/" + name
	}
	switch name {
	case "node_modules", "vendor", ".venv", "venv", "__pycache__":
		return "dependency/" + name
	}
	return ""
}
func agentDirectory(name string) bool {
	switch name {
	case ".claude", ".codex", ".agents", ".cursor", ".gemini", ".github", ".opencode":
		return true
	}
	return false
}

func classify(p, home string) (string, string) {
	// An installation at a home boundary owns its bundled packages, including
	// nested agent markers. Worktree/project containers explicitly delegate to
	// project installations. Nearest markers only supply the fallback agent hint.
	nearest := ""
	for dir := filepath.Clean(p); ; dir = filepath.Dir(dir) {
		agent := filepath.Base(dir)
		if agentDirectory(agent) {
			if nearest == "" {
				nearest = agent
			}
			parent := filepath.Dir(dir)
			atHome := sameDirectoryPath(parent, home) || conventionalHome(parent)
			if atHome && agent != ".github" {
				rel, _ := filepath.Rel(dir, p)
				first := strings.Split(filepath.ToSlash(rel), "/")[0]
				if first == "worktrees" || first == "projects" {
					return "project", nearest
				}
				return "global", agent
			}
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if nearest != "" {
		return "project", nearest
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
