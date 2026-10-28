package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"skill-audit/internal/audit"
	"skill-audit/internal/workflow"
)

func TestSyncMissingOutputDoesNotTransmitNames(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inv := filepath.Join(root, "inventory.json")
	if err = workflow.WriteJSON(inv, workflow.Inventory{Schema: workflow.InventorySchema, Packages: []workflow.Package{{Name: "private-internal-name"}}}); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	t.Setenv("GH_TOKEN", "fixture")
	requests := 0
	http.DefaultTransport = fixtureTransport(func(*http.Request) (*http.Response, error) { requests++; return nil, fmt.Errorf("unexpected request") })
	out, errout := new(bytes.Buffer), new(bytes.Buffer)
	if code := run(context.Background(), []string{"sync", "--inventory", inv, "--search-names"}, out, errout); code != 2 {
		t.Fatal(code)
	}
	if requests != 0 || !bytes.Contains(errout.Bytes(), []byte("requires --out")) {
		t.Fatal("network preceded validation", requests, errout)
	}
}

func TestInterruptedAuditDoesNotRecordBaseline(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inv := filepath.Join(root, "inventory.json")
	state := filepath.Join(root, "state.json")
	if err = workflow.WriteJSON(inv, workflow.Inventory{Schema: workflow.InventorySchema, Packages: []workflow.Package{}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, errout := new(bytes.Buffer), new(bytes.Buffer)
	if code := run(ctx, []string{"audit", "--inventory", inv, "--state", state, "--record", "baseline", "--json"}, out, errout); code != 3 {
		t.Fatal(code, out, errout)
	}
	if _, err = os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("interrupted baseline written", err)
	}
	// A later complete run can still establish the baseline.
	if code := run(context.Background(), []string{"audit", "--inventory", inv, "--state", state, "--record", "baseline", "--json"}, new(bytes.Buffer), errout); code != 0 {
		t.Fatal(code, errout)
	}
	before, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if code := run(ctx, []string{"audit", "--inventory", inv, "--state", state, "--record", "snapshot", "--json"}, new(bytes.Buffer), errout); code != 3 {
		t.Fatal(code)
	}
	after, err := os.ReadFile(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("interrupted snapshot changed state", err)
	}
	if st, err := audit.LoadState(state); err != nil || len(st.Observations) != 1 {
		t.Fatal(st, err)
	}
}
