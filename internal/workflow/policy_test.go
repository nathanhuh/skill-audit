package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"path/filepath"
	"runtime"
	"skill-audit/internal/audit"
	"testing"
)

func TestNonstandardGlobalInstallationAndWorkspaceScopes(t *testing.T) {
	home := temp(t)
	for _, tc := range []struct{ relative, want string }{
		{".codex/superpowers/skills/x", "global"},
		{".claude/community/skills/x", "global"},
		{".codex/worktrees/repo/custom/x", "project"},
		{".claude/projects/repo/custom/x", "project"},
		{".codex/worktrees/repo/.claude/skills/x", "project"},
	} {
		if scope, _ := classify(filepath.Join(home, filepath.FromSlash(tc.relative)), home); scope != tc.want {
			t.Errorf("%s: %s", tc.relative, scope)
		}
	}
}

func TestHomeRootKeepsPriorityOverDeepExternalRoot(t *testing.T) {
	root := temp(t)
	home := filepath.Join(root, "home")
	external := filepath.Join(root, "external", "a", "b", "c", "d")
	put(t, filepath.Join(home, ".claude", "skills", "x", "SKILL.md"), "fixture")
	for i := 0; i < 50; i++ {
		put(t, filepath.Join(external, fmt.Sprintf("file-%02d", i)), "fixture")
	}
	for _, selected := range []string{home, filepath.Join(home, ".claude", "skills")} {
		inv := Discover(context.Background(), []string{external, selected}, home, Limits{Entries: 15, Packages: 10, Depth: 64})
		if len(inv.Packages) != 1 || inv.Packages[0].Scope != "global" {
			t.Fatal("external root starved selected home", inv)
		}
	}
}

func TestRootDepthCountsComponents(t *testing.T) {
	root := string(filepath.Separator)
	for _, tc := range []struct {
		p     string
		depth int
	}{
		{root, 0}, {filepath.Join(root, "opt"), 1}, {filepath.Join(root, "opt", "skills"), 2},
		{filepath.Join(root, "opt", ".."), 0},
	} {
		if depth := rootDepth(tc.p); depth != tc.depth {
			t.Errorf("%q: %d", tc.p, depth)
		}
	}
	if runtime.GOOS == "windows" {
		for _, tc := range []struct {
			p     string
			depth int
		}{{`C:\`, 0}, {`C:\skills`, 1}, {`\\server\share`, 0}, {`\\server\share\skills`, 1}} {
			if depth := rootDepth(tc.p); depth != tc.depth {
				t.Errorf("volume %q: %d", tc.p, depth)
			}
		}
	}

}

func TestInventoryLinkPolicyDoesNotHidePackageGap(t *testing.T) {
	root := temp(t)
	pkg := filepath.Join(root, "project", ".claude", "skills", "x")
	put(t, filepath.Join(pkg, "SKILL.md"), "fixture")
	if err := os.Symlink(pkg, filepath.Join(root, "project", "link")); err != nil {
		t.Skip("fixture symlinks unavailable: ", err)
	}
	store, err := LoadStore("")
	if err != nil {
		t.Fatal(err)
	}
	inspect := func() audit.Report {
		inv := Discover(context.Background(), []string{filepath.Join(root, "project")}, "", DefaultLimits())
		report, err := Audit(context.Background(), inv, store, "", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	report := inspect()
	if report.HasCoverageGaps() || len(report.Diagnostics) == 0 {
		t.Fatal("policy note became failure or disappeared", report)
	}
	if err = audit.SaveObservation(context.Background(), filepath.Join(root, "baseline.json"), report, true); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("SKILL.md", filepath.Join(pkg, "bundled-link")); err != nil {
		t.Fatal(err)
	}
	report = inspect()
	if !report.HasCoverageGaps() || report.Skills[0].Complete {
		t.Fatal("skipped package content treated as complete", report)
	}
	if err = audit.SaveObservation(context.Background(), filepath.Join(root, "partial.json"), report, true); err == nil {
		t.Fatal("package gap baseline accepted")
	}
}

func TestOutputPlacementErrorUsesNeutralRoleAndRejectsLinks(t *testing.T) {
	root := temp(t)
	put(t, filepath.Join(root, "pkg", "SKILL.md"), "fixture")
	if err := WriteJSON(filepath.Join(root, "pkg", "out.json"), map[string]string{}); err == nil || strings.Contains(err.Error(), "state") {
		t.Fatal("misleading output error", err)
	}
	dest := filepath.Join(root, "dest.json")
	put(t, dest, "original")
	link := filepath.Join(root, "link.json")
	if err := os.Symlink(dest, link); err != nil {
		t.Skip("fixture symlinks unavailable: ", err)
	}
	if err := WriteJSON(link, map[string]string{}); err == nil {
		t.Fatal("output symlink followed")
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "original" {
		t.Fatal("link destination changed", string(b), err)
	}
}
