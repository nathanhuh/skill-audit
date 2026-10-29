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
	Budget                            *InventoryBudget
}

func DefaultInventoryOptions() InventoryOptions {
	return InventoryOptions{MaxEntries: 500000, MaxPackages: 2000, MaxDepth: 64, DeviceRoots: DeviceRoots}
}
func InventoryRun(ctx context.Context, o InventoryOptions) Inventory {
	if o.Home != "" {
		o.Home = Abs(o.Home)
	}
	inv := Inventory{Schema: InventorySchema, CreatedAt: time.Now().UTC(), Home: o.Home, Policy: DiscoveryPolicy, Complete: true, Limits: Limits{o.MaxEntries, o.MaxPackages, o.MaxDepth}, Packages: []Package{}}
	explicitRoots := map[string]bool{}
	for _, root := range o.Roots {
		explicitRoots[Abs(root)] = true
	}
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
		if o.Home != "" && Within(p, o.Home) {
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
	rootPriority := func(p string) int {
		if o.Home != "" && Within(o.Home, p) {
			scope, _ := Classify(p, o.Home)
			if scope == "global" {
				return 0
			}
			return 1
		}
		return priority(p)
	}
	ranks := map[string]int{}
	depths := map[string]int{}
	for _, root := range roots {
		ranks[root] = rootPriority(root)
		depths[root] = strings.Count(root, string(filepath.Separator))
	}
	sort.Slice(roots, func(i, j int) bool {
		if explicitRoots[roots[i]] != explicitRoots[roots[j]] {
			return explicitRoots[roots[i]]
		}
		a, b := ranks[roots[i]], ranks[roots[j]]
		if a != b {
			return a < b
		}
		a, b = depths[roots[i]], depths[roots[j]]
		if a != b {
			return a < b
		}
		return roots[i] < roots[j]
	})
	if o.StatePath != "" {
		inv.StatePath = Abs(o.StatePath)
	}
	budget := newInventoryBudget(&inv, o.Budget)
	if inv.outputFailure != nil {
		inv.Diagnostics = append(inv.Diagnostics, diag("inventory_output_budget", "", inv.outputFailure.Error()))
		return inv
	}
	exclusions := map[string]*Exclusion{}
	stopped := ""
	gap := func(code, path, msg string) { budget.diagnostic(diag(code, path, msg)) }
	exclude := func(rule, path string) {
		old := exclusions[rule]
		next := Exclusion{Rule: rule, Examples: []string{}}
		if old != nil {
			next = *old
			next.Examples = append([]string(nil), old.Examples...)
		}
		next.Count++
		if len(next.Examples) < 3 {
			next.Examples = append(next.Examples, path)
		} else {
			next.OmittedExamples++
		}
		if !budget.exclusion(old, next) {
			stopped = "inventory_output_budget"
			gap(stopped, path, "exclusion evidence exceeded inventory output capacity")
			return
		}
		exclusions[rule] = &next
	}

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
	selectedInfo := map[string]os.FileInfo{}
	for _, r := range roots {
		if NoLinks(r) != nil {
			continue
		}
		i, e := os.Lstat(r)
		if e == nil && i.IsDir() && !isLink(i) && exclusionRule(r, i, true) == "" {
			selected[r] = true
			selectedInfo[r] = i
		}
	}
	var seenRoots []struct {
		path string
		info os.FileInfo
	}
	var walk func(string, int, bool)
	walk = func(path string, depth int, root bool) {
		if check() {
			gap(stopped, path, "location not inspected: "+stopped)
			return
		}
		if !root && selected[path] {
			budget.diagnostic(info("root_delegated", path, "covered by its separately selected root: "+path))
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
			for _, selectedPath := range roots {
				si := selectedInfo[selectedPath]
				if selectedPath != path && si != nil && os.SameFile(info, si) {
					budget.diagnostic(Diagnostic{Code: "root_delegated", Path: path, Message: "filesystem alias covered by separately selected root: " + selectedPath})
					return
				}
			}
		}
		rule := exclusionRule(path, info, root)
		if rule != "" {
			if root {
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
		budget.entry()
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
					if !budget.packageMeasurement(p) {
						stopped = "inventory_output_budget"
						gap(stopped, path, "next whole package measurement exceeds inventory output capacity; earlier measurements retained")
						return
					}
					break
				}
			}
		}
		entryRanks := make(map[string]int, len(entries))
		for _, entry := range entries {
			entryRanks[entry.Name()] = priority(filepath.Join(path, entry.Name()))
		}
		sort.SliceStable(entries, func(i, j int) bool {
			a, b := entryRanks[entries[i].Name()], entryRanks[entries[j].Name()]
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
			if os.SameFile(i, s.info) {
				budget.diagnostic(info("root_delegated", root, "filesystem alias of separately selected root: "+s.path))
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seenRoots = append(seenRoots, struct {
			path string
			info os.FileInfo
		}{root, i})
		walk(root, 0, true)
	}
	for _, x := range exclusions {
		inv.Exclusions = append(inv.Exclusions, *x)
	}
	sort.Slice(inv.Exclusions, func(i, j int) bool { return inv.Exclusions[i].Rule < inv.Exclusions[j].Rule })
	sort.Slice(inv.Packages, func(i, j int) bool { return Identity(inv.Packages[i]) < Identity(inv.Packages[j]) })
	budget.finish()
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
