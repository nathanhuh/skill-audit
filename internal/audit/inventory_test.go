package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestD13ExplicitRootsBeforeInjectedDeviceRoot(t *testing.T) {
	root := tempDir(t)
	device := filepath.Join(root, "device")
	home := filepath.Join(device, "home")
	explicit := filepath.Join(home, ".claude")
	fixture(t, explicit, "p/SKILL.md", "# fixture")
	for i := 0; i < 30; i++ {
		fixture(t, home, strings.Repeat("z", i+1), "data")
	}
	o := fixtureOptions(explicit)
	o.Device = true
	o.Home = home
	o.DeviceRoots = func() []string { return []string{device} }
	o.MaxEntries = 8
	inv := InventoryRun(context.Background(), o)
	if len(inv.Packages) != 1 || !inv.Packages[0].Complete {
		t.Fatal(inv)
	}
}

func TestReview2DependencyOmissionRemainsIncompleteLocally(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	g.add("p/node_modules/data", "relevant bytes")
	c := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	if len(c.Skills) != 1 || len(c.Skills[0].Files) != 2 {
		t.Fatal(c)
	}
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	fixture(t, root, "node_modules/data", "relevant bytes")
	p := Measure(context.Background(), root, "")
	if p.Complete || p.Digest != "" || NewMatcher(c).Match(p).Status == "exact" {
		t.Fatal(p)
	}
}

func TestReview3DelegatedCaseAliasIsReported(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, ".Claude/p/SKILL.md", "# fixture")
	actual, alias := filepath.Join(root, ".Claude"), filepath.Join(root, ".claude")
	a, _ := os.Stat(actual)
	b, e := os.Stat(alias)
	if e != nil || !os.SameFile(a, b) {
		t.Skip("requires case-insensitive filesystem")
	}
	o := fixtureOptions(alias)
	o.Device = true
	o.DeviceRoots = func() []string { return []string{root} }
	inv := InventoryRun(context.Background(), o)
	if !inv.Complete || len(inv.Packages) != 1 {
		t.Fatal(inv)
	}
	found := false
	for _, d := range inv.Diagnostics {
		if d.Path == actual && d.Code == "root_delegated" && !d.Incomplete && strings.Contains(d.Message, alias) {
			found = true
		}
	}
	if !found {
		t.Fatal("delegated alias missing from evidence", inv)
	}
}

func TestReview4DuplicateSelectedRootHasEvidence(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, ".claude/skills/p/SKILL.md", "# fixture")
	actual, alias := filepath.Join(root, ".claude/skills"), filepath.Join(root, ".Claude/skills")
	a, _ := os.Stat(actual)
	b, e := os.Stat(alias)
	if e != nil || !os.SameFile(a, b) {
		t.Skip("requires case-insensitive filesystem")
	}
	o := fixtureOptions(actual)
	o.Roots = append(o.Roots, alias)
	inv := InventoryRun(context.Background(), o)
	if !inv.Complete || len(inv.Packages) != 1 || len(inv.Roots) != 2 {
		t.Fatal(inv)
	}
	found := false
	for _, d := range inv.Diagnostics {
		if d.Code == "root_delegated" && d.Path == inv.Roots[1] && strings.Contains(d.Message, inv.Roots[0]) && !d.Incomplete {
			found = true
		}
	}
	if !found {
		t.Fatal("duplicate selected root has no delegation evidence", inv)
	}
}

func TestReviewD10ExplicitMetadataRootIsRejectedVisibly(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, ".git/p/SKILL.md", "# fixture")
	inv := InventoryRun(context.Background(), fixtureOptions(filepath.Join(root, ".git")))
	if inv.Complete || len(inv.Packages) != 0 || !hasCode(inv.Diagnostics, "excluded_root") {
		t.Fatal(inv)
	}
}

func TestReviewD3D6RejectedAliasCannotHideValidRoot(t *testing.T) {
	root := tempDir(t)
	actual := filepath.Join(root, "actual")
	fixture(t, actual, "x/SKILL.md", "# fixture")
	alias := filepath.Join(root, "alias")
	if e := os.Symlink(actual, alias); e != nil {
		t.Skip(e)
	}
	o := fixtureOptions(actual)
	o.Roots = append(o.Roots, filepath.Join(alias, "x"))
	inv := InventoryRun(context.Background(), o)
	if inv.Complete || len(inv.Packages) != 1 || !hasCode(inv.Diagnostics, "link_path") {
		t.Fatal(inv)
	}
}

func TestD4ImplicitMacAliasRuleKey(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS alias scenario")
	}
	info, e := os.Stat(tempDir(t))
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"/tmp", "/etc", "/var"} {
		rule := exclusionRule(p, info, false)
		if rule != "system"+p || strings.Contains(rule, "//") {
			t.Fatal(p, rule)
		}
		if rule = exclusionRule(p, info, true); rule != "" {
			t.Fatal("explicit alias bypasses the no-follow gate", p, rule)
		}
	}
}

func TestD8FiftyThousandMetadataExclusions(t *testing.T) {
	if os.Getenv("SKILL_AUDIT_LARGE_FIXTURES") != "1" {
		t.Skip("set SKILL_AUDIT_LARGE_FIXTURES=1 for the 50,000-entry fixture")
	}
	root := tempDir(t)
	for i := 0; i < 50000; i++ {
		fixture(t, root, fmt.Sprintf("d%05d/.DS_Store", i), "fixture")
	}
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	if !inv.Complete || len(inv.Exclusions) != 1 {
		t.Fatal(inv.Complete, len(inv.Exclusions))
	}
	x := inv.Exclusions[0]
	if x.Count != 50000 || len(x.Examples) != 3 || x.OmittedExamples != 49997 {
		t.Fatal(x)
	}
	b, e := Encode(inv)
	if e != nil || len(b) > 10000 {
		t.Fatal("unbounded exclusion evidence", len(b), e)
	}
}
