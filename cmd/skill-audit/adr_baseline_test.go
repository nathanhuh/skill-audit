package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skill-audit/internal/audit"
	"skill-audit/internal/workflow"
)

func TestADRPartialBaselineB2B4E5(t *testing.T) {
	for _, id := range []string{"B2", "B4"} {
		t.Run(id, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(root, "project")
			pkg := filepath.Join(project, "bar")
			if err = os.MkdirAll(pkg, 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			limits := workflow.DefaultLimits()
			if id == "B2" {
				if err = os.Symlink(pkg, filepath.Join(project, "foo")); err != nil {
					t.Skip("fixture links unavailable:", err)
				}
			} else {
				next := filepath.Join(project, "foo")
				if err = os.Mkdir(next, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(next, "SKILL.md"), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				limits.Packages = 1
			}
			inv := workflow.Discover(context.Background(), []string{project}, "", limits)
			input, state := filepath.Join(root, "inventory.json"), filepath.Join(root, "state.json")
			if err = workflow.WriteJSON(input, inv); err != nil {
				t.Fatal(err)
			}
			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			code := run(context.Background(), []string{"audit", "--inventory", input, "--state", state, "--record", "baseline", "--json"}, stdout, stderr)
			var r audit.Report
			if code != 2 || json.Unmarshal(stdout.Bytes(), &r) != nil || r.Recording.Status != audit.RecordingRecorded || !r.HasCoverageGaps() {
				t.Fatal(code, stdout, stderr)
			}
			st, err := audit.LoadState(state)
			if err != nil || len(st.Anchors) != 1 || st.Observations[0].Report.Discovery.Policy != workflow.DiscoveryPolicy {
				t.Fatal(st, err)
			}
		})
	}
}

func TestADRScannerFailureStillRecordsContentB3(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "bar")
	if err = os.Mkdir(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	inv := workflow.Discover(context.Background(), []string{pkg}, "", workflow.DefaultLimits())
	input, state := filepath.Join(root, "inventory.json"), filepath.Join(root, "state.json")
	if err = workflow.WriteJSON(input, inv); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := run(context.Background(), []string{"audit", "--inventory", input, "--state", state, "--record", "baseline", "--xray", filepath.Join(root, "unavailable-scanner"), "--json"}, stdout, stderr)
	var r audit.Report
	if code != 2 || json.Unmarshal(stdout.Bytes(), &r) != nil || r.Recording.Status != audit.RecordingRecorded || r.Skills[0].Evaluations[1].Status != "unavailable" {
		t.Fatal(code, stdout, stderr)
	}
	st, err := audit.LoadState(state)
	if err != nil || len(st.Anchors) != 1 {
		t.Fatal(st, err)
	}
}
