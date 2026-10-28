package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUserPackagesBeforeSystemBudgetExhaustion(t *testing.T) {
	for _, container := range []string{"Users", "home"} {
		t.Run(container, func(t *testing.T) {
			root := temp(t)
			home := filepath.Join(root, container, "developer")
			global := filepath.Join(home, ".claude", "skills", "global")
			project := filepath.Join(home, "projects", "one", ".claude", "skills", "project")
			put(t, filepath.Join(global, "SKILL.md"), "global fixture")
			put(t, filepath.Join(project, "SKILL.md"), "project fixture")
			for i := 0; i < 50; i++ {
				put(t, filepath.Join(root, "Applications", fmt.Sprintf("file-%03d", i)), "noise")
			}
			// This bounded fixture reproduces lexical starvation without a device scan.
			limits := Limits{Entries: 20, Packages: 10, Depth: 64}
			inv := Discover(context.Background(), []string{root}, home, limits)
			if len(inv.Packages) != 2 {
				t.Fatalf("user packages starved: %+v", inv)
			}
			limited := false
			for _, d := range inv.Diagnostics {
				if d.Code == "entry_limit" {
					limited = true
				}
			}
			if !limited {
				t.Fatal("expected visible exhaustion after user skills")
			}
			again := Discover(context.Background(), []string{root}, home, limits)
			if !reflect.DeepEqual(inv, again) {
				t.Fatal("priority traversal is not deterministic")
			}
			inv = Discover(context.Background(), []string{root}, "", limits)
			if len(inv.Packages) != 2 {
				t.Fatal("conventional user container not prioritized without home", inv)
			}
		})
	}
}

func TestInventoryCancellationExplainsAllUnvisitedRoots(t *testing.T) {
	root := temp(t)
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	put(t, filepath.Join(a, "pkg", "SKILL.md"), "fixture")
	put(t, filepath.Join(b, "pkg", "SKILL.md"), "fixture")
	for _, tc := range []struct {
		name, code  string
		makeContext func() (context.Context, context.CancelFunc)
	}{
		{"cancel", "inventory_canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}},
		{"timeout", "inventory_timeout", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Unix(0, 0))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.makeContext()
			defer cancel()
			inv := Discover(ctx, []string{b, a}, "", DefaultLimits())
			if inv.Entries != 0 || len(inv.Packages) != 0 || len(inv.Diagnostics) != 2 {
				t.Fatal(inv)
			}
			for i, d := range inv.Diagnostics {
				if d.Code != tc.code || d.Path != []string{a, b}[i] || !d.Incomplete || !strings.Contains(d.Message, "not started") {
					t.Fatal(inv.Diagnostics)
				}
			}
		})
	}
}

func TestOverlappingRootsCountEntriesOnce(t *testing.T) {
	root := temp(t)
	project := filepath.Join(root, "project")
	for i := 0; i < 30; i++ {
		put(t, filepath.Join(project, fmt.Sprintf("file-%02d", i)), "fixture")
	}
	put(t, filepath.Join(project, ".claude", "skills", "pkg", "SKILL.md"), "fixture")
	one := Discover(context.Background(), []string{root}, "", DefaultLimits())
	overlap := Discover(context.Background(), []string{project, root, project}, "", DefaultLimits())
	if one.Entries != overlap.Entries || !reflect.DeepEqual(one.Packages, overlap.Packages) {
		t.Fatal("overlap duplicated work", one, overlap)
	}
}

func TestZeroRequestBudgetNeverMakesARequest(t *testing.T) {
	c := NewClient("")
	c.MaxRequests = 0
	_, err := c.Sync(context.Background(), "owner/repo", "HEAD", 1)
	if err == nil || !strings.Contains(err.Error(), "request budget") || c.Requests != 0 {
		t.Fatal(err, c.Requests)
	}
}

// Cancel deterministically during traversal, without sleeps or a timing race.
// The precise number of checks is not part of the production contract.
type interruptingContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *interruptingContext) Err() error {
	c.checks++
	if c.checks == 20 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCancellationDuringTraversalKeepsEvidenceAndCause(t *testing.T) {
	root := temp(t)
	home := filepath.Join(root, "home")
	later := filepath.Join(root, "later")
	put(t, filepath.Join(home, ".claude", "skills", "example", "SKILL.md"), "fixture")
	for i := 0; i < 50; i++ {
		put(t, filepath.Join(home, fmt.Sprintf("z-noise-%02d", i)), "fixture")
	}
	put(t, filepath.Join(later, "SKILL.md"), "fixture")
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &interruptingContext{Context: base, cancel: cancel}
	inv := Discover(ctx, []string{later, home}, home, DefaultLimits())
	if len(inv.Packages) != 1 || !inv.Packages[0].Complete {
		t.Fatal("partial results lost", inv)
	}
	interrupted, pending := false, false
	for _, d := range inv.Diagnostics {
		if d.Code != "inventory_canceled" {
			t.Fatal("cancellation confused with another failure", d)
		}
		interrupted = interrupted || d.Path == home && strings.Contains(d.Message, "interrupted")
		pending = pending || d.Path == later && strings.Contains(d.Message, "not started")
	}
	if !interrupted || !pending {
		t.Fatal(inv.Diagnostics)
	}
}

func TestPriorityTraversalKeepsDepthAndPackageLimits(t *testing.T) {
	root := temp(t)
	home := filepath.Join(root, "z-home")
	later := filepath.Join(root, "a-other")
	for _, name := range []string{"a", "b"} {
		put(t, filepath.Join(home, ".claude", "skills", name, "SKILL.md"), "fixture")
	}
	put(t, filepath.Join(later, "pkg", "SKILL.md"), "fixture")
	inv := Discover(context.Background(), []string{later, home}, home, Limits{Entries: 100, Packages: 1, Depth: 64})
	if len(inv.Packages) != 1 || !strings.HasPrefix(inv.Packages[0].Path, home) {
		t.Fatal("home root lost priority", inv)
	}
	pending := false
	for _, d := range inv.Diagnostics {
		pending = pending || d.Path == later && d.Code == "package_limit" && strings.Contains(d.Message, "not started")
	}
	if !pending {
		t.Fatal("unsearched root omitted after package limit", inv.Diagnostics)
	}
	inv = Discover(context.Background(), []string{home}, home, Limits{Entries: 100, Packages: 10, Depth: 1})
	if len(inv.Packages) != 0 {
		t.Fatal("depth limit bypassed", inv)
	}
	limited := false
	for _, d := range inv.Diagnostics {
		limited = limited || d.Code == "depth_limit"
	}
	if !limited {
		t.Fatal("depth gap omitted", inv)
	}
}
