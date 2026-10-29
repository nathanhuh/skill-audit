package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"skill-audit/internal/audit"
)

func Audit(ctx context.Context, inventory Inventory, store Store, xray string, timeout time.Duration) (audit.Report, error) {
	r := audit.Report{Recording: &audit.Recording{Status: audit.RecordingNotRequested}, Schema: audit.ReportSchema, Roots: []audit.Root{}, Skills: []audit.Skill{}, Lineage: []audit.Lineage{}, Diagnostics: append([]audit.Diagnostic{}, inventory.Diagnostics...)}
	if inventory.Schema != InventorySchema && inventory.Schema != "skill-audit/inventory/v2" {
		return r, fmt.Errorf("unsupported inventory schema")
	}
	r.Discovery = &audit.Discovery{Roots: append([]string{}, inventory.Roots...), Home: inventory.Home, Policy: inventory.Policy, Limits: audit.TraversalLimits(inventory.Limits), Exclusions: append([]audit.Exclusion{}, inventory.Exclusions...)}
	if r.Discovery.Policy == "" {
		r.Discovery.Policy = "unknown (legacy inventory)"
	}
	catalog, e := store.Catalog()
	if e != nil {
		return r, e
	}
	r.CatalogVersion = catalog.Version
	r.Diagnostics = append(r.Diagnostics, store.Diagnostics...)
	matcher := audit.NewMatcher(catalog)
	seen := map[string]bool{}
	for _, pkg := range inventory.Packages {
		if !filepath.IsAbs(pkg.Path) || filepath.Clean(pkg.Path) != pkg.Path || seen[pkg.Path] || (pkg.Scope != "global" && pkg.Scope != "project" && pkg.Scope != "machine") {
			return r, fmt.Errorf("invalid/duplicate inventory identity")
		}
		seen[pkg.Path] = true
	}
	scanner := audit.NewXRayScanner(xray, timeout)
	defer scanner.Close()
	packages := append([]Package{}, inventory.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Path < packages[j].Path })
	for _, pkg := range packages {
		if ctx.Err() != nil {
			break
		}
		r.Roots = append(r.Roots, audit.Root{Scope: pkg.Scope, Path: pkg.Path, Explicit: true, Status: "inventoried"})
		s := audit.Inspect(ctx, pkg.Scope, pkg.Path)
		if !pkg.Complete || !s.Complete || s.Digest != pkg.Digest {
			s.Diagnostics = append(s.Diagnostics, audit.Diagnostic{Scope: pkg.Scope, Path: pkg.Path, Code: "inventory_drift", Message: "contents changed or were incomplete since inventory; current bytes scanned, refresh inventory before treating this as a reproducible audit", Incomplete: true})
		}
		s.Correlation = matcher.Correlate(s)
		s.Origins = append([]audit.Candidate{}, s.Correlation.Candidates...)
		audit.LocalScan(&s)
		scanner.Scan(ctx, &s)
		// Evidence and hashes survive; raw local content is needed only while scanning.
		s.Data = nil
		r.Skills = append(r.Skills, s)
	}
	if ctx.Err() != nil {
		r.Diagnostics = append(r.Diagnostics, audit.Diagnostic{Code: "audit_canceled", Message: ctx.Err().Error(), Incomplete: true})
	}
	return r, nil
}
