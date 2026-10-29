package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"skill-audit/internal/audit"
)

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
		fmt.Fprintln(out, "usage: skill-audit inventory|search|sync|audit [options]")
		return 2
	}
	command := args[0]
	if command == "--help" || command == "help" {
		fmt.Fprintln(out, "usage: skill-audit inventory|search|sync|audit [options]")
		return 0
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
		f.StringVar(&inventory, "inventory", "", "inventory for opt-in name search")
		f.BoolVar(&names, "search-names", false, "allow transmission of inventory names (max 10)")
		f.StringVar(&registry, "registry-url", "", "Skills.sh v1 endpoint")
		f.IntVar(&limit, "limit", 20, "search hits or packages per repository target")
		if command == "sync" {
			f.Var(&repos, "repo", "explicit owner/repo (repeatable)")
			f.StringVar(&ref, "ref", "HEAD", "reference resolved to immutable commit")
			f.StringVar(&searchPath, "search", "", "saved search")
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
			for _, p := range i.Packages {
				printDiagnostics(out, p.Diagnostics)
			}
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
		if inventory == "" || scanTimeout <= 0 || (record != "" && record != "baseline" && record != "snapshot") || (record != "" && state == "") {
			return bad(fmt.Errorf("audit requires --inventory; recording requires --state and baseline|snapshot; scan timeout must be positive"))
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
			if search.Schema != audit.SearchSchema {
				return bad(fmt.Errorf("unsupported search schema"))
			}
		}
		if len(queries) > 0 {
			next := client.Search(queries, registry, limit)
			if search.Schema != "" {
				next.Hits = append(search.Hits, next.Hits...)
				next.Diagnostics = append(search.Diagnostics, next.Diagnostics...)
				next.Complete = next.Complete && search.Complete
			}
			search = next
		}
		if command == "search" {
			if len(queries) == 0 {
				return bad(fmt.Errorf("search requires --query or --inventory --search-names"))
			}
			ok := emit(search, func() {
				fmt.Fprintf(out, "search hits=%d complete=%t\n", len(search.Hits), search.Complete)
				printDiagnostics(out, search.Diagnostics)
			})
			if !ok || !search.Complete {
				return 2
			}
			return 0
		}
		if len(repos) == 0 && len(search.Hits) == 0 {
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
func printRecording(w io.Writer, r *audit.Recording) {
	fmt.Fprintf(w, "recording=%s path=%q message=%q\n", r.Status, r.Path, r.Message)
}
func printDiagnostics(w io.Writer, ds []audit.Diagnostic) {
	for _, d := range ds {
		fmt.Fprintf(w, "%q path=%q message=%q\n", d.Code, d.Path, d.Message)
	}
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
			fmt.Fprintf(w, "scanner=%s version=%q status=%s message=%q analyzed=%d outside_scope=%d\n", e.Scanner, e.Version, e.Status, e.Message, len(e.Analyzed), len(e.OutsideScope))
		}
		for _, f := range s.Findings {
			fmt.Fprintf(w, "%s/%q %s %q:%d change=%s evidence=%q\n", f.Scanner, f.RuleID, f.Severity, f.File, f.Line, f.Change, f.Evidence)
		}
		printDiagnostics(w, s.Diagnostics)
	}
	printExclusions(w, r.Exclusions)
	printDiagnostics(w, r.Diagnostics)
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
