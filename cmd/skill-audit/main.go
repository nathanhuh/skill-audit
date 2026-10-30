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
	"runtime/debug"
	"strings"
	"time"

	"github.com/nathanhuh/skill-audit/internal/audit"
)

// version is set by release builds with -ldflags "-X main.version=v0.1.0".
var version string

// buildVersion prefers the release stamp, then the module version recorded by
// `go install …@version`, and reports "dev" for local builds.
func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const usage = "usage: skill-audit scan | inventory|search|sync|audit [options]\n" +
	"quick start: run skill-audit scan from a project folder (scans ~/.claude and that folder, saves reports under ~/.skill-audit/reports)"

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(parent context.Context, args []string, out, errOut io.Writer) int {
	delivery := &checkedWriter{w: out}
	out = delivery
	if len(args) == 0 {
		fmt.Fprintln(out, usage)
		return 2
	}
	command := args[0]
	if command == "--help" || command == "help" {
		fmt.Fprintln(out, usage)
		return 0
	}
	if command == "--version" || command == "version" {
		fmt.Fprintln(out, "skill-audit", buildVersion())
		return 0
	}
	if command == "scan" {
		return runScan(parent, args[1:], out, errOut)
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(errOut)
	timeout := f.Duration("timeout", 2*time.Minute, "operation deadline")
	output := f.String("out", "", "output JSON file")
	asJSON := f.Bool("json", false, "print JSON")
	var roots, repos stringsFlag
	var device bool
	var home, inventory, catalog, state, record, xray, query, registry, searchPath, ref string
	var names bool
	var limit int
	var scanTimeout time.Duration
	opts := audit.DefaultInventoryOptions()
	switch command {
	case "inventory":
		f.Var(&roots, "root", "selected root (repeatable)")
		f.BoolVar(&device, "device", false, "include device roots (default without roots)")
		home, _ = os.UserHomeDir()
		f.StringVar(&home, "home", home, "installation ownership hint")
		f.StringVar(&state, "state", "", "designated state file excluded from discovery")
		f.IntVar(&opts.MaxEntries, "max-entries", opts.MaxEntries, "entry budget")
		f.IntVar(&opts.MaxPackages, "max-packages", opts.MaxPackages, "package budget")
		f.IntVar(&opts.MaxDepth, "max-depth", opts.MaxDepth, "depth budget")
	case "audit":
		f.StringVar(&inventory, "inventory", "", "saved inventory (required)")
		f.StringVar(&catalog, "catalog", "", "external reference catalog")
		f.StringVar(&state, "state", "", "lineage file")
		f.StringVar(&record, "record", "", "baseline or snapshot")
		f.StringVar(&xray, "xray", "", "absolute trusted Skill X-Ray executable")
		f.DurationVar(&scanTimeout, "scan-timeout", 30*time.Second, "version probe and each scan phase budget")
	case "search", "sync":
		f.StringVar(&query, "query", "", "explicit public query")
		f.StringVar(&searchPath, "search", "", "saved search to combine with explicit queries")
		f.StringVar(&inventory, "inventory", "", "inventory for opt-in name search")
		f.BoolVar(&names, "search-names", false, "allow transmission of inventory names (max 10)")
		f.StringVar(&registry, "registry-url", "", "Skills.sh v1 endpoint")
		f.IntVar(&limit, "limit", 20, "search hits or packages per repository target")
		if command == "sync" {
			f.Var(&repos, "repo", "explicit owner/repo (repeatable)")
			f.StringVar(&ref, "ref", "HEAD", "reference resolved to immutable commit")
		}
	default:
		fmt.Fprintln(errOut, "unknown command")
		return 2
	}
	if e := f.Parse(args[1:]); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return 2
	}
	bad := func(e error) int { fmt.Fprintln(errOut, e); return 2 }
	if f.NArg() != 0 || *timeout <= 0 {
		return bad(fmt.Errorf("unexpected arguments or non-positive timeout"))
	}
	ctx, cancel := context.WithTimeout(parent, *timeout)
	defer cancel()
	inputs := []string{inventory, catalog, searchPath, state}
	var observed []audit.Package
	emit := func(v any, text func()) bool {
		success := true
		if *output != "" {
			e := audit.OutputSafe(*output, inputs, observed)
			if e == nil {
				e = audit.WriteJSON(context.Background(), *output, v, observed...)
			}
			if e != nil {
				fmt.Fprintln(errOut, e)
				success = false
			}
		}
		if *asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if e := enc.Encode(v); e != nil {
				fmt.Fprintln(errOut, e)
				return false
			}
		} else {
			text()
		}
		if delivery.err != nil {
			fmt.Fprintln(errOut, "output delivery failed")
			return false
		}
		return success
	}
	if *output != "" {
		if e := audit.OutputSafe(*output, inputs, nil); e != nil {
			return bad(e)
		}
	}
	switch command {
	case "inventory":
		opts.Roots = roots
		opts.Device = device
		opts.Home = home
		opts.StatePath = state
		if e := audit.ValidateInventoryOptions(opts); e != nil {
			return bad(e)
		}
		if state != "" {
			if e := audit.OutputSafe(state, nil, nil); e != nil {
				return bad(e)
			}
		}
		i := audit.InventoryRun(ctx, opts)
		if e := i.OutputError(); e != nil {
			return bad(e)
		}
		observed = i.Packages
		ok := emit(i, func() {
			fmt.Fprintf(out, "inventory packages=%d complete=%t entries=%d\n", len(i.Packages), i.Complete, i.Entries)
			for _, p := range i.Packages {
				fmt.Fprintf(out, "%s %q name=%q complete=%t\n", p.Scope, p.Path, p.Name, p.Complete)
			}
			var packageDiagnostics []audit.Diagnostic
			for _, p := range i.Packages {
				packageDiagnostics = append(packageDiagnostics, p.Diagnostics...)
			}
			printDiagnostics(out, packageDiagnostics)
			if i.DiagnosticsTruncated {
				printOmissions(out, i.OmittedDiagnostics)
			}
			printExclusions(out, i.Exclusions)
			printDiagnostics(out, i.Diagnostics)
		})
		if !ok || !i.Complete {
			return 2
		}
		for _, p := range i.Packages {
			if !p.Complete || len(p.Diagnostics) > 0 {
				return 2
			}
		}
		return 0
	case "audit":
		switch {
		case inventory == "":
			return bad(fmt.Errorf("audit requires --inventory FILE (create it with skill-audit inventory --out FILE)"))
		case record != "" && record != "baseline" && record != "snapshot":
			return bad(fmt.Errorf("--record must be baseline or snapshot, not %q", record))
		case record != "" && state == "":
			return bad(fmt.Errorf("--record %s requires --state FILE", record))
		case scanTimeout <= 0:
			return bad(fmt.Errorf("--scan-timeout must be positive"))
		}
		i, e := audit.ReadInventory(inventory)
		observed = i.Packages
		if e != nil {
			return bad(e)
		}
		if state != "" {
			if e = audit.OutputSafe(state, []string{inventory, catalog, *output}, i.Packages); e != nil {
				return bad(e)
			}
		}
		if *output != "" {
			if e = audit.OutputSafe(*output, inputs, i.Packages); e != nil {
				return bad(e)
			}
		}
		var c audit.Catalog
		if catalog == "" {
			c, e = audit.BuiltinCatalog()
		} else {
			c, e = audit.ReadCatalog(catalog)
		}
		if e != nil {
			return bad(e)
		}
		r := audit.AuditRun(ctx, i, c, audit.AuditOptions{StatePath: state, Record: record, Xray: xray, Timeout: *timeout, ScanTimeout: scanTimeout})
		if record != "" {
			printRecording(errOut, r.Recording)
		}
		ok := emit(r, func() {
			printReport(out, r)
			if record != "" {
				printRecording(out, r.Recording)
			}
		})
		exit := audit.Exit(r)
		if !ok && exit == 0 {
			return 2
		}
		return exit
	case "search", "sync":
		if limit <= 0 || limit > 100 {
			return bad(fmt.Errorf("limit must be 1–100"))
		}
		if command == "sync" && *output == "" {
			return bad(fmt.Errorf("sync requires --out"))
		}
		queries, e := audit.Queries(query, inventory, names)
		if e != nil {
			return bad(e)
		}
		client := audit.NewAcquirer(ctx)
		var search audit.Search
		if searchPath != "" {
			if e = audit.Decode(searchPath, audit.StoreLimit, &search); e != nil {
				return bad(e)
			}
		}
		if len(queries) > 0 {
			next := client.Search(queries, registry, limit)
			search = audit.MergeSearch(search, next)
		}
		if command == "search" {
			if len(queries) == 0 {
				return bad(fmt.Errorf("search requires --query or --inventory --search-names"))
			}
			ok := emit(search, func() {
				fmt.Fprintf(out, "search hits=%d complete=%t\n", len(search.Hits()), search.Complete())
				printDiagnostics(out, search.Diagnostics())
			})
			if !ok || !search.Complete() {
				return 2
			}
			return 0
		}
		if len(repos) == 0 && len(search.Hits()) == 0 {
			return bad(fmt.Errorf("sync requires repositories or search targets"))
		}
		c := client.Sync(repos, ref, search, limit)
		ok := emit(c, func() {
			fmt.Fprintf(out, "sync references=%d complete=%t\n", len(c.Skills), c.Complete)
			printDiagnostics(out, c.Diagnostics)
		})
		if !ok || !c.Complete {
			return 2
		}
		return 0
	}
	return 2
}

// runScan inventories ~/.claude and the current folder, then audits offline,
// saving both files in a new folder under ~/.skill-audit/reports (ADR-0016).
func runScan(ctx context.Context, args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("scan", flag.ContinueOnError)
	f.SetOutput(errOut)
	timeout := f.Duration("timeout", 2*time.Minute, "deadline for each step")
	asJSON := f.Bool("json", false, "print the report JSON instead of text")
	if e := f.Parse(args); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(errOut, "scan takes only --timeout and --json; use inventory and audit for custom roots, outputs or recording")
		return 2
	}
	if f.NArg() != 0 {
		fmt.Fprintln(errOut, "scan takes no arguments; use inventory and audit for custom roots, outputs or recording")
		return 2
	}
	home, e := os.UserHomeDir()
	if e != nil {
		fmt.Fprintln(errOut, "scan needs a home folder:", e)
		return 2
	}
	cwd, e := os.Getwd()
	if e == nil {
		cwd, e = filepath.EvalSymlinks(cwd)
	}
	if e != nil {
		fmt.Fprintln(errOut, "scan cannot resolve the current folder:", e)
		return 2
	}
	roots, note := scanRoots(home, cwd)
	notes := out
	if *asJSON {
		notes = errOut
	}
	if len(roots) == 0 {
		fmt.Fprintln(errOut, "nothing to scan:", note)
		return 2
	}
	fmt.Fprintf(notes, "scan roots=%q\n", roots)
	if note != "" {
		fmt.Fprintln(notes, "note:", note)
	}
	dir, e := reportDir(home, time.Now().UTC())
	if e != nil {
		fmt.Fprintln(errOut, "scan cannot create its report folder:", e)
		return 2
	}
	inv, rep := filepath.Join(dir, "inventory.json"), filepath.Join(dir, "report.json")
	inventoryArgs := []string{"inventory", "--home", home, "--timeout", timeout.String(), "--out", inv}
	for _, r := range roots {
		inventoryArgs = append(inventoryArgs, "--root", r)
	}
	run(ctx, inventoryArgs, io.Discard, errOut)
	if _, e := os.Stat(inv); e != nil {
		return 2
	}
	auditArgs := []string{"audit", "--inventory", inv, "--timeout", timeout.String(), "--out", rep}
	if *asJSON {
		auditArgs = append(auditArgs, "--json")
	}
	code := run(ctx, auditArgs, out, errOut)
	if _, e := os.Stat(rep); e == nil {
		fmt.Fprintf(notes, "saved inventory=%q report=%q\n", inv, rep)
	}
	return code
}

// scanRoots selects ~/.claude and the current folder (ADR-0016 rule 2).
func scanRoots(home, cwd string) ([]string, string) {
	var roots []string
	claude := filepath.Join(home, ".claude")
	if s, e := os.Stat(claude); e == nil && s.IsDir() {
		roots = append(roots, claude)
	}
	if real, e := filepath.EvalSymlinks(home); e == nil {
		home = real
	}
	switch {
	case inside(cwd, home):
		return roots, "the current folder contains your home folder; run scan from a project folder to include its project skills"
	case inside(filepath.Join(home, ".claude"), cwd), inside(filepath.Join(home, ".skill-audit"), cwd):
		return roots, "the current folder is inside ~/.claude or ~/.skill-audit; run scan from a project folder to include its project skills"
	}
	return append(roots, cwd), ""
}

// inside reports whether child is parent or lies below it.
func inside(parent, child string) bool {
	r, e := filepath.Rel(parent, child)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

// reportDir creates a new owner-only folder named by UTC time; it never
// reuses an existing one.
func reportDir(home string, now time.Time) (string, error) {
	base := filepath.Join(home, ".skill-audit", "reports")
	if e := os.MkdirAll(base, 0700); e != nil {
		return "", e
	}
	name := now.Format("20060102T150405Z")
	for i := 1; ; i++ {
		dir := filepath.Join(base, name)
		if i > 1 {
			dir = fmt.Sprintf("%s-%d", dir, i)
		}
		e := os.Mkdir(dir, 0700)
		if e == nil {
			return dir, nil
		}
		if !os.IsExist(e) || i == 100 {
			return "", e
		}
	}
}

func printRecording(w io.Writer, r *audit.Recording) {
	fmt.Fprintf(w, "recording=%s path=%q message=%q\n", r.Status, r.Path, r.Message)
}

// printDiagnostics prints one line per code in first-seen order (ADR-0015 L1).
// A lone diagnostic keeps its path and message; a repeated code shows a count,
// up to three example paths and the first message. JSON keeps every record.
func printDiagnostics(w io.Writer, ds []audit.Diagnostic) {
	var order []string
	groups := map[string][]audit.Diagnostic{}
	for _, d := range ds {
		if _, ok := groups[d.Code]; !ok {
			order = append(order, d.Code)
		}
		groups[d.Code] = append(groups[d.Code], d)
	}
	for _, code := range order {
		g := groups[code]
		if len(g) == 1 {
			fmt.Fprintf(w, "%q path=%q message=%q\n", code, g[0].Path, g[0].Message)
			continue
		}
		examples := []string{}
		for _, d := range g[:min(3, len(g))] {
			examples = append(examples, d.Path)
		}
		fmt.Fprintf(w, "%q count=%d examples=%q omitted_examples=%d message=%q\n", code, len(g), examples, len(g)-len(examples), g[0].Message)
	}
}

// informative reports whether an evaluation earns a text line (ADR-0015 L2).
func informative(e audit.Evaluation) bool {
	return e.Status != "disabled" && !(e.Scanner == "skill-audit" && e.Status == "completed_without_findings")
}

// printSummary ends audit text with one counting line (ADR-0015 L3).
func printSummary(w io.Writer, r audit.Report) {
	scopes, statuses, findings := map[string]int{}, map[string]int{}, 0
	for _, s := range r.Skills {
		scopes[s.Scope]++
		statuses[s.Correlation.Status]++
		findings += len(s.Findings)
	}
	fmt.Fprintf(w, "summary skills=%d global=%d project=%d machine=%d exact=%d possible_modified=%d ambiguous=%d unknown=%d findings=%d complete=%t\n",
		len(r.Skills), scopes["global"], scopes["project"], scopes["machine"], statuses["exact"], statuses["possible_modified"], statuses["ambiguous"], statuses["unknown"], findings, r.Complete)
}
func printReport(w io.Writer, r audit.Report) {
	if r.DiagnosticsTruncated {
		printOmissions(w, r.OmittedDiagnostics)
	}
	if r.BudgetStop != "" {
		fmt.Fprintf(w, "inventory_budget_stop=%q\n", r.BudgetStop)
	}
	fmt.Fprintf(w, "audit skills=%d complete=%t discovery_complete=%t\n", len(r.Skills), r.Complete, r.DiscoveryComplete)
	for _, s := range r.Skills {
		fmt.Fprintf(w, "%s %q correlation=%s lineage=%s\n", s.Scope, s.Path, s.Correlation.Status, s.Lineage.Status)
		for _, c := range s.Correlation.Candidates {
			fmt.Fprintf(w, "candidate=%q repository=%q commit=%q upstream_path=%q basis=%q score=%.3f shared=%d union=%d\n", c.Reference.ID, c.Reference.Repository, c.Reference.Commit, c.Reference.UpstreamPath, c.Basis, c.Score, c.Shared, c.Union)
			if c.UpstreamComparison == "unavailable" {
				fmt.Fprintln(w, "upstream_comparison=unavailable")
			}
			for _, x := range c.UpstreamChanges {
				fmt.Fprintf(w, "upstream_%s=%q\n", x.Kind, x.Path)
			}
		}
		for _, p := range s.Lineage.Added {
			fmt.Fprintf(w, "added=%q\n", p)
		}
		for _, p := range s.Lineage.Removed {
			fmt.Fprintf(w, "removed=%q\n", p)
		}
		for _, p := range s.Lineage.Modified {
			fmt.Fprintf(w, "modified=%q\n", p)
		}
		for _, e := range s.Evaluations {
			if !informative(e) {
				continue
			}
			fmt.Fprintf(w, "scanner=%s version=%q status=%s message=%q analyzed=%d outside_scope=%d\n", e.Scanner, e.Version, e.Status, e.Message, len(e.Analyzed), len(e.OutsideScope))
		}
		for _, f := range s.Findings {
			fmt.Fprintf(w, "%s/%q %s %q:%d change=%s evidence=%q\n", f.Scanner, f.RuleID, f.Severity, f.File, f.Line, f.Change, f.Evidence)
		}
		printDiagnostics(w, s.Diagnostics)
	}
	printExclusions(w, r.Exclusions)
	printDiagnostics(w, r.Diagnostics)
	printSummary(w, r)
}

func printExclusions(w io.Writer, xs []audit.Exclusion) {
	for _, x := range xs {
		fmt.Fprintf(w, "exclusion=%q count=%d examples=%q omitted_examples=%d\n", x.Rule, x.Count, x.Examples, x.OmittedExamples)
	}
}

type checkedWriter struct {
	w   io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	n, e := w.w.Write(p)
	if e != nil && w.err == nil {
		w.err = e
	}
	return n, e
}

func printOmissions(w io.Writer, counts map[string]int) {
	b, _ := json.Marshal(counts)
	fmt.Fprintf(w, "diagnostics_truncated=true omitted_by_code=%s\n", b)
}
