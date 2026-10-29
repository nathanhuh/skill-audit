//go:build windows

package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Native runtime evidence is required: compilation cannot verify either test.
func TestD7P4NativeWindowsJunctionIsNotFollowed(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "target/SKILL.md", "# fixture")
	// Fixed arguments and a fixture working directory avoid shell interpolation of paths.
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", "alias", "target")
	cmd.Dir = root
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Skipf("junction fixture unavailable: %v: %s", e, output)
	}
	alias := filepath.Join(root, "alias")
	info, e := os.Lstat(alias)
	if e != nil || !isLink(info) {
		t.Fatal("junction not recognized", e)
	}
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	if inv.Complete || len(inv.Packages) != 1 || !hasCode(inv.Diagnostics, "symlink_skipped") {
		t.Fatal(inv)
	}
}
func TestP3NativeWindowsCancellationStopsDescendants(t *testing.T) {
	source, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(tempDir(t), "scan-descendant.exe")
	if e = os.WriteFile(exe, data, 0700); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		os.WriteFile(exe+".stopchild", []byte("stop"), 0600)
		until := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(until) {
			if _, err := os.Stat(exe + ".heartbeat.done"); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	s := NewScanner(exe, 10*time.Second, 2*time.Second)
	defer s.Close()
	evaluation, _ := s.Scan(context.Background(), measuredFixture(t))
	if evaluation.Status != "failed" || !strings.Contains(evaluation.Message, "timeout") {
		t.Fatal(evaluation)
	}
	before, e := os.ReadFile(exe + ".heartbeat")
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(150 * time.Millisecond)
	after, e := os.ReadFile(exe + ".heartbeat")
	if e != nil {
		t.Fatal(e)
	}
	if string(before) != string(after) {
		t.Fatal("scanner descendant survived cancellation; Windows Job Object support remains pending")
	}
}
