package workflow

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNestedWorktreesAndGitHubScope(t *testing.T) {
	home := filepath.Join(temp(t), "home")
	for _, tc := range []struct{ rel, scope, agent string }{
		{".codex/skills/global", "global", ".codex"},
		{".codex/plugins/cache/pkg", "global", ".codex"},
		{".codex/worktrees/repo/.claude/skills/pkg", "project", ".claude"},
		{".codex/worktrees/repo/.codex/skills/pkg", "project", ".codex"},
		{".codex/worktrees/repo/custom/pkg", "project", ".codex"},
		{".github/skills/pkg", "project", ".github"},
	} {
		if scope, agent := classify(filepath.Join(home, filepath.FromSlash(tc.rel)), home); scope != tc.scope || agent != tc.agent {
			t.Errorf("%s: %s/%s", tc.rel, scope, agent)
		}
	}
	for _, base := range []string{"/Users/alice", "/home/alice", "/root", "C:/Users/alice"} {
		for _, selected := range []string{base, "/unrelated"} {
			if scope, _ := classify(base+"/.github/skills/pkg", selected); scope != "project" {
				t.Errorf("github scope depends on home: %s %s %s", base, selected, scope)
			}
		}
	}
}

func TestExplicitRootsOverrideParentTraversalCuts(t *testing.T) {
	for _, tc := range []struct {
		name, child string
		depth       int
	}{
		{"excluded", "vendor", 64},
		{"depth cut", "a/b", 1},
		{"admitted parent with deeper child", "a", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := temp(t)
			child := filepath.Join(root, filepath.FromSlash(tc.child))
			put(t, filepath.Join(child, "nested", "SKILL.md"), "fixture")
			inv := Discover(context.Background(), []string{root, child}, root, Limits{Entries: 100, Packages: 10, Depth: tc.depth})
			if len(inv.Packages) != 1 {
				t.Fatal("explicit root lost to parent traversal", inv)
			}
		})
	}
}

func TestRelativeHomeHintKeepsGlobalInstallation(t *testing.T) {
	home := temp(t)
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, home)
	if err != nil {
		t.Skip("fixture home is on a different volume: ", err)
	}
	scope, _ := classify(filepath.Join(home, ".claude", "skills", "pkg"), relative)
	if scope != "global" {
		t.Fatal("relative home hint changed scope", scope)
	}
}
