//go:build windows

package audit

import (
	"context"
	"path/filepath"
	"testing"
)

// Compilation is recorded separately from running this on native NTFS.
func TestNativeWindowsStateReplacement(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "p/SKILL.md", "# native fixture")
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	path := filepath.Join(tempDir(t), "state.json")
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: path, Record: "baseline"})
	if r.Recording.Status != "recorded" {
		t.Fatal(r.Recording)
	}
	r = AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: path, Record: "snapshot"})
	if r.Recording.Status != "recorded" {
		t.Fatal(r.Recording)
	}
	state, err := ReadState(path)
	if err != nil || len(state.Observations) != 2 {
		t.Fatal(state, err)
	}
}
