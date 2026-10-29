package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type InventoryOptions struct {
	Roots                             []string
	Device                            bool
	Home, StatePath                   string
	MaxEntries, MaxPackages, MaxDepth int
	DeviceRoots                       func() []string
}

func DefaultInventoryOptions() InventoryOptions {
	return InventoryOptions{MaxEntries: 500000, MaxPackages: 2000, MaxDepth: 64, DeviceRoots: DeviceRoots}
}
func InventoryRun(ctx context.Context, o InventoryOptions) Inventory {
	if o.Home != "" {
		o.Home = Abs(o.Home)
	}
	inv := Inventory{Schema: InventorySchema, CreatedAt: time.Now().UTC(), Home: o.Home, Policy: DiscoveryPolicy, Complete: true, Limits: Limits{o.MaxEntries, o.MaxPackages, o.MaxDepth}, Packages: []Package{}}
	roots := append([]string(nil), o.Roots...)
	if o.Device || len(roots) == 0 {
		f := o.DeviceRoots
		if f == nil {
			f = DeviceRoots
		}
		roots = append(roots, f()...)
	}
	for _, r := range roots {
		inv.Roots = append(inv.Roots, Abs(r))
	}
	sort.Strings(inv.Roots)
	inv.Roots = unique(inv.Roots)
	roots = append([]string(nil), inv.Roots...)
	priority := func(p string) int {
		if o.Home != "" && Within(p, Abs(o.Home)) {
			return 0
		}
		if isAgent(filepath.Base(p)) {
			return 1
		}
		switch filepath.Base(p) {
		case "Users", "home", "root":
			return 2
		}
		return 3
	}
	sort.Slice(roots, func(i, j int) bool {
		a, b := priority(roots[i]), priority(roots[j])
		if a != b {
			return a < b
		}
		a, b = strings.Count(roots[i], string(filepath.Separator)), strings.Count(roots[j], string(filepath.Separator))
		if a != b {
			return a < b
		}
		return roots[i] < roots[j]
	})
	if o.StatePath != "" {
		inv.StatePath = Abs(o.StatePath)
	}
	exclusions := map[string]*Exclusion{}
	exclude := func(rule, path string) {
		x := exclusions[rule]
		if x == nil {
			x = &Exclusion{Rule: rule, Examples: []string{}}
			exclusions[rule] = x
		}
		x.Count++
		if len(x.Examples) < 3 {
			x.Examples = append(x.Examples, path)
		} else {
			x.OmittedExamples++
		}
	}
	gap := func(code, path, msg string) {
		inv.Complete = false
		inv.Diagnostics = append(inv.Diagnostics, diag(code, path, msg))
	}
	stopped := ""
	check := func() bool {
		if ctx.Err() != nil {
			stopped = "inventory_canceled"
			if ctx.Err() == context.DeadlineExceeded {
				stopped = "inventory_timeout"
			}
		}
		if stopped == "" && inv.Entries >= o.MaxEntries {
			stopped = "entry_limit"
		}
		return stopped != ""
	}
	selected := map[string]bool{}
	for _, r := range roots {
		selected[r] = true
	}
	selectedInfo := map[string]os.FileInfo{}
	for _, r := range roots {
		if i, e := os.Lstat(r); e == nil && i.IsDir() && !isLink(i) {
			selectedInfo[r] = i
		}
	}
	var seenRoots []os.FileInfo
	var walk func(string, int, bool)
	walk = func(path string, depth int, root bool) {
		if check() {
			gap(stopped, path, "location not inspected: "+stopped)
			return
		}
		if !root && selected[path] {
			return
		}
		if path == inv.StatePath {
			exclude("state/location", path)
			return
		}
		info, e := os.Lstat(path)
		if e != nil {
			gap("walk_error", path, e.Error())
			return
		}
		if !root && info.IsDir() {
			for selectedPath, si := range selectedInfo {
				if selectedPath != path && os.SameFile(info, si) {
					return
				}
			}
		}
		rule := exclusionRule(path, info, root)
		if rule != "" {
			if root && strings.HasPrefix(rule, "system/") {
				gap("excluded_root", path, "selected root rejected by "+rule)
			} else {
				exclude(rule, path)
			}
			return
		}
		if isLink(info) {
			gap("symlink_skipped", path, "link not followed")
			return
		}
		inv.Entries++
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				gap("special_file", path, "special entry not inspected")
			}
			return
		}
		if depth > o.MaxDepth {
			gap("depth_limit", path, "depth limit reached")
			return
		}
		entries, e := os.ReadDir(path)
		if e != nil {
			gap("walk_error", path, e.Error())
			return
		}
		for _, x := range entries {
			if x.Name() == "SKILL.md" {
				m, e := os.Lstat(filepath.Join(path, x.Name()))
				if e == nil && m.Mode().IsRegular() {
					if len(inv.Packages) >= o.MaxPackages {
						stopped = "package_limit"
						gap(stopped, path, "package limit reached")
						return
					}
					p := Measure(ctx, path, o.Home)
					p.Data = nil
					inv.Packages = append(inv.Packages, p)
					break
				}
			}
		}
		sort.SliceStable(entries, func(i, j int) bool {
			a, b := priority(filepath.Join(path, entries[i].Name())), priority(filepath.Join(path, entries[j].Name()))
			if a != b {
				return a < b
			}
			return entries[i].Name() < entries[j].Name()
		})
		for _, x := range entries {
			walk(filepath.Join(path, x.Name()), depth+1, false)
			if stopped != "" {
				return
			}
		}
	}
	for _, root := range roots {
		if check() {
			gap(stopped, root, "selected root not started: "+stopped)
			continue
		}
		if e := NoLinks(root); e != nil {
			gap("link_path", root, e.Error())
			continue
		}
		i, e := os.Lstat(root)
		if e != nil {
			gap("root_error", root, e.Error())
			continue
		}
		if !i.IsDir() {
			gap("root_error", root, "selected root is not a directory")
			continue
		}
		duplicate := false
		for _, s := range seenRoots {
			if os.SameFile(i, s) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seenRoots = append(seenRoots, i)
		walk(root, 0, true)
	}
	for _, x := range exclusions {
		inv.Exclusions = append(inv.Exclusions, *x)
	}
	sort.Slice(inv.Exclusions, func(i, j int) bool { return inv.Exclusions[i].Rule < inv.Exclusions[j].Rule })
	sort.Slice(inv.Packages, func(i, j int) bool { return Identity(inv.Packages[i]) < Identity(inv.Packages[j]) })
	return inv
}
func unique(s []string) []string {
	out := s[:0]
	for _, v := range s {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
func ValidateInventoryOptions(o InventoryOptions) error {
	if o.MaxEntries <= 0 || o.MaxPackages <= 0 || o.MaxDepth <= 0 {
		return fmt.Errorf("inventory limits must be positive")
	}
	return nil
}
