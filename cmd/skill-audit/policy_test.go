package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skill-audit/internal/audit"
	"skill-audit/internal/workflow"
)

func TestProjectExclusionsStillAllowBaseline(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "project")
	pkg := filepath.Join(project, ".claude", "skills", "example")
	for _, p := range []string{filepath.Join(pkg, "SKILL.md"), filepath.Join(project, ".git", "config"), filepath.Join(project, "node_modules", "dependency", "file")} {
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(p, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inv := filepath.Join(root, "inventory.json")
	state := filepath.Join(root, "state.json")
	out, errout := new(bytes.Buffer), new(bytes.Buffer)
	if code := run(context.Background(), []string{"inventory", "--root", project, "--out", inv}, out, errout); code != 0 {
		t.Fatal("routine exclusions block inventory", code, out, errout)
	}
	if code := run(context.Background(), []string{"audit", "--inventory", inv, "--state", state, "--record", "baseline", "--json"}, out, errout); code != 0 {
		t.Fatal("routine exclusions block baseline", code, out, errout)
	}
	st, err := audit.LoadState(state)
	if err != nil || len(st.Observations) != 1 || len(st.Observations[0].Report.Diagnostics) < 2 {
		t.Fatal(st, err)
	}
}

func TestRecordFailuresAreDistinct(t *testing.T) {
	for _, kind := range []string{"lock", "already exists", "corrupt snapshot"} {
		t.Run(kind, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			inv := filepath.Join(root, "inventory.json")
			state := filepath.Join(root, "state.json")
			if err = workflow.WriteJSON(inv, workflow.Inventory{Schema: workflow.InventorySchema, Packages: []workflow.Package{}}); err != nil {
				t.Fatal(err)
			}
			record := "baseline"
			if kind == "lock" {
				if err = os.WriteFile(state+".lock", nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "already exists" {
				if code := run(context.Background(), []string{"audit", "--inventory", inv, "--state", state, "--record", "baseline"}, new(bytes.Buffer), new(bytes.Buffer)); code != 0 {
					t.Fatal(code)
				}
			}
			if kind == "corrupt snapshot" {
				record = "snapshot"
				if err = os.WriteFile(state, []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, errout := new(bytes.Buffer), new(bytes.Buffer)
			code := run(context.Background(), []string{"audit", "--inventory", inv, "--state", state, "--record", record, "--json"}, out, errout)
			var report audit.Report
			if code != 3 || !strings.Contains(errout.String(), "state not recorded") || json.Unmarshal(out.Bytes(), &report) != nil {
				t.Fatal("record failure hidden", code, out, errout)
			}
			found := false
			for _, d := range report.Diagnostics {
				found = found || d.Code == "state_not_recorded"
			}
			if !found {
				t.Fatal(report)
			}
		})
	}
}
