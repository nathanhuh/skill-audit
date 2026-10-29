package workflow

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"skill-audit/internal/audit"
)

func TestADRInstallationOwnershipS1S2(t *testing.T) {
	root := temp(t)
	home := filepath.Join(root, "home")
	for _, tc := range []struct{ id, rel, scope, agent string }{
		{"S1", ".claude/plugins/marketplaces/m/.claude/skills/x", "global", ".claude"},
		{"S1-community", ".codex/superpowers/.claude/skills/x", "global", ".codex"},
		{"S1-direct", ".claude/skills/x", "global", ".claude"},
		{"S2", ".codex/worktrees/repo/.claude/skills/x", "project", ".claude"},
		{"S2-projects", ".claude/projects/repo/.codex/skills/x", "project", ".codex"},
		{"S2-no-inner-marker", ".codex/worktrees/repo/custom/x", "project", ".codex"},
		{"github", ".github/skills/x", "project", ".github"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			pkg := filepath.Join(home, filepath.FromSlash(tc.rel))
			put(t, filepath.Join(pkg, "SKILL.md"), "---\nname: same-name\n---\nfixture")
			inv := Discover(context.Background(), []string{pkg}, home, DefaultLimits())
			if len(inv.Packages) != 1 || inv.Packages[0].Scope != tc.scope || inv.Packages[0].Agent != tc.agent {
				t.Fatal(inv)
			}
		})
	}
	store, err := LoadStore("")
	if err != nil {
		t.Fatal(err)
	}
	inv := Discover(context.Background(), []string{home}, home, DefaultLimits())
	report, err := Audit(context.Background(), inv, store, "", time.Second)
	if err != nil || len(report.Skills) != 7 {
		t.Fatal(report, err)
	}
	state := filepath.Join(root, "state.json")
	if err = audit.SaveObservation(context.Background(), state, report, true); err != nil {
		t.Fatal(err)
	}
	st, err := audit.LoadState(state)
	if err != nil {
		t.Fatal(err)
	}
	audit.Compare(&report, st)
	for _, l := range report.Lineage {
		if l.Status != "unchanged" {
			t.Fatal(l)
		}
	}
	// A stored old scope is historical evidence; comparisons never rewrite it.
	st.Observations[0].Report.Skills[0].Scope = "machine"
	st.Anchors[0].Scope = "machine"
	report.Lineage = nil
	audit.Compare(&report, st)
	seen := map[string]bool{}
	for _, l := range report.Lineage {
		seen[l.Status] = true
	}
	if !seen["not_in_baseline"] || !seen["missing_or_undiscovered"] {
		t.Fatal(report.Lineage)
	}
}
