package workflow

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReviewExplicitMacAliasesReachNoFollowDiagnostic(t *testing.T) {
	for _, alias := range []string{"/tmp", "/etc", "/var"} {
		p := alias + "/skills"
		if excludedOnOS(p, "skills", p, "darwin") {
			t.Fatalf("explicit alias root hidden by policy: %s", p)
		}
		if !excludedOnOS(alias, filepath.Base(alias), "/", "darwin") {
			t.Fatal("discovered alias no longer excluded", alias)
		}
	}
	if runtime.GOOS != "darwin" {
		t.Skip("native macOS alias diagnostic")
	}

	// A fixture under the real /private/tmp tree is readable; its /tmp alias is not.
	fixture, err := os.MkdirTemp("/private/tmp", "skill-audit-alias-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fixture)
	if err = os.WriteFile(filepath.Join(fixture, "SKILL.md"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join("/tmp", filepath.Base(fixture))
	inv := Discover(context.Background(), []string{alias}, "", DefaultLimits())
	if len(inv.Diagnostics) != 1 || inv.Diagnostics[0].Code != "symlink" || len(inv.Exclusions) != 0 {
		t.Fatal(inv)
	}
	inv = Discover(context.Background(), []string{fixture}, "", DefaultLimits())
	if len(inv.Diagnostics) != 0 || len(inv.Packages) != 1 || !inv.Packages[0].Complete {
		t.Fatal(inv)
	}
}

func TestReviewSystemExclusionsUseStableRule(t *testing.T) {
	for _, path := range []string{"/proc", "/proc/a", "/proc/b/deep"} {
		if got := exclusionRuleOnOS(path, filepath.Base(path), path, "linux"); got != "system/proc" {
			t.Fatal(path, got)
		}
	}
}
