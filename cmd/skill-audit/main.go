package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skill-audit/internal/audit"
	"skill-audit/internal/workflow"
)

type many []string

func (m *many) String() string     { return fmt.Sprint([]string(*m)) }
func (m *many) Set(s string) error { *m = append(*m, s); return nil }
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(ctx context.Context, args []string, out, errout io.Writer) int {
	fail := func(e error) int { fmt.Fprintf(errout, "skill-audit: %q\n", e.Error()); return 2 }
	if len(args) == 0 {
		return fail(fmt.Errorf("usage: skill-audit {inventory|search|sync|audit} --help"))
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	var roots, repos many
	var device, searchNames, asJSON bool
	var home, output, inventoryPath, catalogPath, searchFile, query, registry, ref, xray, state, record string
	var limit = 20
	var maxEntries, maxPackages, maxDepth int
	var deadline time.Duration
	f.StringVar(&output, "out", "", "save versioned JSON; parent must exist")
	f.DurationVar(&deadline, "timeout", 2*time.Minute, "operation deadline; also per external scan")
	switch args[0] {
	case "inventory":
		homeDefault, _ := os.UserHomeDir()
		f.Var(&roots, "root", "inventory search root; repeatable")
		f.BoolVar(&device, "device", false, "inventory all accessible device roots, excluding documented subtrees")
		f.StringVar(&home, "home", homeDefault, "home for global/project classification")
		f.IntVar(&maxEntries, "max-entries", 500000, "inventory entry budget")
		f.IntVar(&maxPackages, "max-packages", 2000, "inventory package budget")
		f.IntVar(&maxDepth, "max-depth", 64, "inventory relative directory depth budget")
	case "audit":
		f.StringVar(&inventoryPath, "inventory", "", "saved inventory JSON for offline audit")
		f.StringVar(&catalogPath, "catalog", "", "optional saved catalog; defaults to two pinned Anthropic skills")
		f.StringVar(&xray, "xray", "", "optional trusted Skill X-Ray absolute executable path")
		f.StringVar(&state, "state", "", "optional baseline comparison JSON outside skill packages")
		f.StringVar(&record, "record", "", "baseline or snapshot; requires --state")
		f.BoolVar(&asJSON, "json", false, "JSON rather than readable output")
	case "search", "sync":
		f.StringVar(&inventoryPath, "inventory", "", "saved inventory for opt-in name search")
		f.BoolVar(&searchNames, "search-names", false, "explicitly send names from --inventory to public search, at most 10 distinct names")
		f.StringVar(&query, "query", "", "explicit public search query; sent to chosen service")
		f.StringVar(&registry, "registry-url", "", "optional Skills.sh-v1-compatible HTTPS search endpoint")
		f.IntVar(&limit, "limit", 20, "search results or packages per repository (1–100)")
		if args[0] == "sync" {
			f.Var(&repos, "repo", "sync public owner/repo; repeatable")
			f.StringVar(&searchFile, "search", "", "saved search JSON to import")
			f.StringVar(&ref, "ref", "HEAD", "ref resolved to exact commit")
		}
	default:
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
	if e := f.Parse(args[1:]); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments"))
	}
	if args[0] == "sync" && output == "" {
		return fail(fmt.Errorf("sync requires --out catalog file"))
	}
	if deadline <= 0 || limit < 1 || limit > 100 {
		return fail(fmt.Errorf("invalid timeout/limit"))
	}
	checkPaths := func() error {
		return validateFilePaths(
			fileOption{"--out", output, true},
			fileOption{"--state", state, record != ""},
			fileOption{"--inventory", inventoryPath, false},
			fileOption{"--catalog", catalogPath, false},
			fileOption{"--search", searchFile, false},
		)
	}
	if e := checkPaths(); e != nil {
		return fail(e)
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	emit := func(v any) error {
		if output != "" {
			if e := checkPaths(); e != nil {
				return e
			}
			return workflow.WriteJSON(output, v)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch args[0] {
	case "inventory":
		if maxEntries < 1 || maxPackages < 1 || maxDepth < 1 {
			return fail(fmt.Errorf("inventory limits must be positive"))
		}
		if device {
			roots = append(roots, workflow.DeviceRoots()...)
		}
		if len(roots) == 0 {
			return fail(fmt.Errorf("choose --device or at least one --root"))
		}
		inv := workflow.Discover(ctx, roots, home, workflow.Limits{Entries: maxEntries, Packages: maxPackages, Depth: maxDepth})
		if output != "" {
			absolute, e := filepath.Abs(output)
			if e != nil {
				return fail(e)
			}
			packages := []audit.Root{}
			for _, p := range inv.Packages {
				packages = append(packages, audit.Root{Path: p.Path})
			}
			if e = audit.StateOutside(absolute, packages); e != nil {
				return fail(e)
			}
		}
		if e := emit(inv); e != nil {
			return fail(e)
		}
		for _, d := range inv.Diagnostics {
			if d.Incomplete {
				return 2
			}
		}
		return 0
	case "search", "sync":
		token := os.Getenv("GH_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		client := workflow.NewClient(token)
		targets := []workflow.Hit{}
		explicit := map[string]bool{}
		for _, repo := range repos {
			explicit[repo] = true
			targets = append(targets, workflow.Hit{Repository: repo})
		}
		searches := []workflow.Search{}
		if searchFile != "" {
			var saved workflow.Search
			if e := workflow.ReadJSON(searchFile, &saved); e != nil {
				return fail(e)
			}
			if saved.Schema != "skill-audit/search/v2" {
				return fail(fmt.Errorf("unsupported search schema"))
			}
			searches = append(searches, saved)
		}
		queries := []string{}
		if query != "" {
			queries = append(queries, query)
		}
		if searchNames {
			if inventoryPath == "" {
				return fail(fmt.Errorf("--search-names requires --inventory"))
			}
			var inv workflow.Inventory
			if e := workflow.ReadJSON(inventoryPath, &inv); e != nil {
				return fail(e)
			}
			if inv.Schema != workflow.InventorySchema {
				return fail(fmt.Errorf("unsupported inventory schema"))
			}
			seen := map[string]bool{}
			for _, p := range inv.Packages {
				if p.Name != "" && !seen[p.Name] {
					seen[p.Name] = true
					queries = append(queries, p.Name)
				}
			}
			sort.Strings(queries)
			if len(queries) > 10 {
				return fail(fmt.Errorf("more than 10 search names; narrow inventory or choose explicit --query"))
			}
			fmt.Fprintln(errout, "Search-names enabled: transmitting only recorded skill names to the selected public search service.")
		}
		for _, q := range queries {
			var result workflow.Search
			var e error
			if registry != "" {
				result, e = workflow.SearchRegistry(ctx, registry, os.Getenv("SKILL_AUDIT_REGISTRY_TOKEN"), q, limit)
			} else {
				result, e = client.Search(ctx, q, limit)
			}
			if e != nil {
				return fail(e)
			}
			searches = append(searches, result)
		}
		combined := workflow.Search{Schema: "skill-audit/search/v2", Source: "github-code-search", Query: strings.Join(queries, "; "), Hits: []workflow.Hit{}, Diagnostics: []audit.Diagnostic{}}
		if registry != "" {
			combined.Source = "skills-sh-v1"
		}
		for _, s := range searches {
			combined.Hits = append(combined.Hits, s.Hits...)
			combined.Diagnostics = append(combined.Diagnostics, s.Diagnostics...)
			combined.Incomplete = combined.Incomplete || s.Incomplete
		}
		if args[0] == "search" {
			if len(searches) == 0 {
				return fail(fmt.Errorf("provide --query or --inventory with --search-names"))
			}
			if e := emit(combined); e != nil {
				return fail(e)
			}
			if combined.Incomplete {
				return 2
			}
			return 0
		}
		targets = append(targets, combined.Hits...)
		if len(targets) == 0 {
			return fail(fmt.Errorf("provide --repo, --search, --query, or --search-names"))
		}
		store := workflow.EmptyStore()
		store.Diagnostics = append(store.Diagnostics, combined.Diagnostics...)
		if combined.Incomplete {
			store.Diagnostics = append(store.Diagnostics, audit.Diagnostic{Code: "search_incomplete", Message: "candidate search incomplete/limited", Incomplete: true})
		}
		seen := map[string]bool{}
		entrySeen := map[string]bool{}
		sort.Slice(targets, func(i, j int) bool {
			// Explicit full imports get first use of the shared request budget.
			a := explicit[targets[i].Repository] && targets[i].Path == "" && targets[i].Hint == ""
			b := explicit[targets[j].Repository] && targets[j].Path == "" && targets[j].Hint == ""
			if a != b {
				return a
			}
			return targets[i].Repository+"\x00"+targets[i].Path+"\x00"+targets[i].Hint < targets[j].Repository+"\x00"+targets[j].Path+"\x00"+targets[j].Hint
		})
		for _, target := range targets {
			key := target.Repository + "\x00" + target.Path + "\x00" + target.Hint
			if seen[key] {
				continue
			}
			seen[key] = true
			s, e := client.SyncTarget(ctx, target, ref, limit)
			if e != nil {
				store.Diagnostics = append(store.Diagnostics, audit.Diagnostic{Path: target.Repository, Code: "sync_error", Message: e.Error(), Incomplete: true})
				continue
			}
			for _, entry := range s.Entries {
				if !entrySeen[entry.Reference.ID] {
					entrySeen[entry.Reference.ID] = true
					store.Entries = append(store.Entries, entry)
				}
			}
			store.Diagnostics = append(store.Diagnostics, s.Diagnostics...)
			for k, b := range s.Attribution {
				store.Attribution[k] = b
			}
		}
		sort.Slice(store.Entries, func(i, j int) bool { return store.Entries[i].Reference.ID < store.Entries[j].Reference.ID })
		if e := emit(store); e != nil {
			return fail(e)
		}
		if len(store.Diagnostics) > 0 {
			return 2
		}
		return 0
	case "audit":
		if inventoryPath == "" {
			return fail(fmt.Errorf("--inventory is required"))
		}
		if record != "" && (state == "" || (record != "baseline" && record != "snapshot")) {
			return fail(fmt.Errorf("--record baseline|snapshot requires --state"))
		}
		var inv workflow.Inventory
		if e := workflow.ReadJSON(inventoryPath, &inv); e != nil {
			return fail(e)
		}
		store, e := workflow.LoadStore(catalogPath)
		if e != nil {
			return fail(e)
		}
		r, e := workflow.Audit(ctx, inv, store, xray, deadline)
		if e != nil {
			return fail(e)
		}
		if state != "" {
			absolute, e := filepath.Abs(state)
			if e != nil {
				return fail(e)
			}
			state = absolute
			if e = audit.StateOutside(state, r.Roots); e != nil {
				return fail(e)
			}
			if record != "baseline" {
				st, e := audit.LoadState(state)
				if e != nil {
					return fail(e)
				}
				audit.Compare(&r, st)
			}
			if record != "" {
				if e = checkPaths(); e != nil {
					return fail(e)
				}
				if e = audit.SaveObservation(ctx, state, r, record == "baseline"); e != nil {
					r.Diagnostics = append(r.Diagnostics, audit.Diagnostic{Path: state, Code: "state_not_recorded", Message: e.Error(), Incomplete: true})
				}
			}
		}
		if output != "" || asJSON {
			if e = emit(r); e != nil {
				return fail(e)
			}
		} else {
			audit.Print(out, r)
		}
		return r.ExitCode()
	default:
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
}
