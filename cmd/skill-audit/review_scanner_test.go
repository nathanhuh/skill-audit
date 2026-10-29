package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skill-audit/internal/audit"
	"skill-audit/internal/workflow"
)

func TestReviewHungScannerStillRecordsAllPackages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executable uses /bin/sh; native Windows helper remains pending")
	}
	for _, phase := range []string{"version", "scan"} {
		t.Run(phase, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			packages := filepath.Join(root, "packages")
			for _, name := range []string{"a", "b"} {
				p := filepath.Join(packages, name)
				if err = os.MkdirAll(p, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			inv := workflow.Discover(context.Background(), []string{packages}, "", workflow.DefaultLimits())
			input, state := filepath.Join(root, "inventory.json"), filepath.Join(root, "state.json")
			if err = workflow.WriteJSON(input, inv); err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(root, "fake-scanner")
			script := "#!/bin/sh\nif [ \"$1\" = " + phase + " ]; then sleep 10; fi\nif [ \"$1\" = version ]; then printf 'skill-xray 0.1.0\\n'; fi\n"
			if err = os.WriteFile(exe, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			code := run(context.Background(), []string{"audit", "--inventory", input, "--state", state, "--record", "baseline", "--xray", exe, "--timeout", "2s", "--json"}, stdout, stderr)
			var r audit.Report
			if code != 2 || json.Unmarshal(stdout.Bytes(), &r) != nil || r.Recording.Status != audit.RecordingRecorded || len(r.Skills) != 2 {
				t.Fatal(code, stdout, stderr)
			}
			for _, s := range r.Skills {
				if !s.Complete || s.Evaluations[1].Status != "failed" {
					t.Fatal(s)
				}
			}
			st, err := audit.LoadState(state)
			if err != nil || len(st.Observations[0].Report.Skills) != 2 {
				t.Fatal(st, err)
			}
		})
	}
}
