package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cleanSARIF = `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"skill-xray","version":"0.1.0"}},"invocations":[{"executionSuccessful":true}],"properties":{"coverage":"no-reported-gap"},"results":[]}]}`

func scannerFixture(t *testing.T) *Scanner {
	t.Helper()
	exe := fixture(t, tempDir(t), "scanner", "fake executable data")
	return NewScanner(exe, 2*time.Second, 100*time.Millisecond)
}
func measuredFixture(t *testing.T) Package {
	t.Helper()
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# data")
	return Measure(context.Background(), root, "")
}
func TestX1X2B3StopAndAnchor(t *testing.T) {
	for _, hangProbe := range []bool{true, false} {
		t.Run(fmt.Sprint(hangProbe), func(t *testing.T) {
			s := scannerFixture(t)
			defer s.Close()
			probes, scans := 0, 0
			s.process = func(ctx context.Context, _ string, args []string) (string, int, error) {
				if args[0] == "version" {
					probes++
					if !hangProbe {
						return "skill-xray 0.1.0", 0, nil
					}
				} else {
					scans++
				}
				<-ctx.Done()
				return "", -1, ctx.Err()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			p := measuredFixture(t)
			var first string
			for i := 0; i < 25; i++ {
				e, _ := s.Scan(ctx, p)
				if e.Status != "failed" {
					t.Fatal(e)
				}
				if i == 0 {
					first = e.Message
				} else if e.Message != first {
					t.Fatal("failure cause replaced")
				}
			}
			if probes != 1 || scans != map[bool]int{true: 0, false: 1}[hangProbe] {
				t.Fatal(probes, scans)
			}
			r := Report{Schema: ReportSchema, CreatedAt: time.Now(), DiscoveryComplete: true, Skills: []Skill{{Package: p}}}
			if rec := Record(ctx, filepath.Join(tempDir(t), "state.json"), "baseline", r, nil, nil); rec.Status != "recorded" {
				t.Fatal(rec)
			}
		})
	}
}
func TestX3X4VisibleBudget(t *testing.T) {
	s := scannerFixture(t)
	s.timeout = time.Minute
	s.reserve = time.Second
	defer s.Close()
	calls := 0
	s.process = func(ctx context.Context, _ string, args []string) (string, int, error) {
		calls++
		deadline, _ := ctx.Deadline()
		if left := time.Until(deadline); left < 59*time.Second {
			t.Fatalf("hidden phase cap: %s", left)
		}
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	e, _ := s.Scan(ctx, measuredFixture(t))
	if !Completed(e) || calls != 2 {
		t.Fatal(e, calls)
	}
}
func TestX6SharedProbeDeadlineAndX8NoScanRetry(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	attempts, scans := 0, 0
	var deadline time.Time
	s.process = func(ctx context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			attempts++
			d, _ := ctx.Deadline()
			if attempts == 1 {
				deadline = d
				return "", 75, errors.New("temporary")
			}
			if !d.Equal(deadline) {
				t.Fatal("retry reset deadline")
			}
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if err := os.WriteFile(args[2], []byte(cleanSARIF), 0600); err != nil {
			t.Fatal(err)
		}
		return "", 75, errors.New("temporary scan failure")
	}
	e, _ := s.Scan(context.Background(), measuredFixture(t))
	if attempts != 2 || scans != 1 || e.Status != "partial" {
		t.Fatal(e, attempts)
	}
}
func TestX7LaunchOnceAndX9Reserve(t *testing.T) {
	s := scannerFixture(t)
	n := 0
	s.process = func(context.Context, string, []string) (string, int, error) {
		n++
		return "", -1, errors.New("launch failure")
	}
	for i := 0; i < 3; i++ {
		e, _ := s.Scan(context.Background(), Package{})
		if e.Status != "failed" {
			t.Fatal(e)
		}
	}
	if n != 1 {
		t.Fatal(n)
	}
	s = scannerFixture(t)
	s.reserve = time.Hour
	s.process = func(context.Context, string, []string) (string, int, error) {
		t.Fatal("launched within reserve")
		return "", 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, _ := s.Scan(ctx, Package{})
	if !strings.Contains(e.Message, "reserved") {
		t.Fatal(e)
	}
}
func TestX10OperationTimeoutRefusesRecording(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	r := Report{Schema: ReportSchema, DiscoveryComplete: true}
	rec := Record(ctx, filepath.Join(tempDir(t), "state"), "baseline", r, nil, nil)
	if rec.Status != "not_recorded" {
		t.Fatal(rec)
	}
}
func TestT3T4RealProcessStagingAndSARIF(t *testing.T) {
	// The test executable doubles as a portable fake scanner. No shell is required.
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(tempDir(t), "fake scanner.exe")
	if err = os.WriteFile(exe, data, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_TOKEN", "fixture-secret-should-not-reach-scanner")
	p := measuredFixture(t)
	s := NewScanner(exe, 10*time.Second, 2*time.Second)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e, f := s.Scan(ctx, p)
	if !Completed(e) || len(f) != 0 {
		t.Fatal(e, f)
	}
	log, err := os.ReadFile(exe + ".log")
	if err != nil || string(log) != "version\nscan\n" {
		t.Fatal(string(log), err)
	}
}
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "scanner-child" {

		until := time.Now().Add(10 * time.Second)
		for time.Now().Before(until) {
			if _, e := os.Stat(os.Args[3]); e == nil {
				os.WriteFile(os.Args[2]+".done", []byte("done"), 0600)
				os.Exit(0)
			}
			os.WriteFile(os.Args[2], []byte(fmt.Sprint(time.Now().UnixNano())), 0600)
			time.Sleep(25 * time.Millisecond)
		}
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "scan") {
		exe, _ := os.Executable()
		f, _ := os.OpenFile(exe+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		fmt.Fprintln(f, os.Args[1])
		f.Close()
		if os.Getenv("GH_TOKEN") != "" || os.Getenv("GITHUB_TOKEN") != "" {
			os.Exit(90)
		}
		if strings.Contains(filepath.Base(exe), "probe-hang") {
			time.Sleep(5 * time.Second)
		}
		if os.Args[1] == "version" {
			fmt.Println("skill-xray 0.1.0")
			os.Exit(0)
		}

		if strings.Contains(filepath.Base(exe), "scan-descendant") {
			child := exec.Command(exe, "scanner-child", exe+".heartbeat", exe+".stopchild")
			if e := child.Start(); e != nil {
				os.Exit(97)
			}
			child.Process.Release()
			time.Sleep(5 * time.Second)
			os.Exit(98)
		}
		if strings.Contains(filepath.Base(exe), "scan-hang") {
			time.Sleep(5 * time.Second)
		}
		if len(os.Args) != 7 || os.Args[2] != "--output" || os.Args[4] != "--llm=false" || os.Args[5] != "--" || !filepath.IsAbs(os.Args[6]) {
			os.Exit(91)
		}
		if b, e := os.ReadFile(filepath.Join(os.Args[6], "SKILL.md")); e != nil || len(b) == 0 {
			os.Exit(92)
		}
		if strings.Contains(filepath.Base(exe), "scan-flood") {
			fmt.Print(strings.Repeat("x", 70<<10))
			fmt.Fprint(os.Stderr, strings.Repeat("y", 70<<10))
		}
		report, code := cleanSARIF, 0
		if strings.Contains(filepath.Base(exe), "scan-findings-exit1") {
			report = review3FindingSARIF
			code = 1
		}
		if strings.Contains(filepath.Base(exe), "scan-partial-exit2") {
			report = strings.Replace(cleanSARIF, "no-reported-gap", "incomplete", 1)
			code = 2
		}
		if e := os.WriteFile(os.Args[3], []byte(report), 0600); e != nil {
			os.Exit(93)
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}
func TestSARIFAttributionAndCoverage(t *testing.T) {
	result := `[{"ruleId":"XR-1","message":{"text":"original"},"properties":{"effectiveSeverity":"medium","sxv":"execution","coverage":"incomplete","evidence":{"line":"data"}},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"run.sh"},"region":{"startLine":3}}}]}]`
	b := strings.Replace(cleanSARIF, `"results":[]`, `"results":`+result, 1)
	var e Evaluation
	findings, err := parseSARIF([]byte(b), &e)
	if err != nil || e.Status != "partial" || len(findings) != 1 || findings[0].Scanner != "skill-xray" || findings[0].Severity != "medium" || findings[0].Kind != "security" || len(findings[0].Original) == 0 {
		t.Fatal(e, findings, err)
	}
	_, err = parseSARIF([]byte(cleanSARIF), &e)
	if err != nil || !Completed(e) {
		t.Fatal(e, err)
	}
	for _, bad := range []string{`{}`, strings.Replace(cleanSARIF, "skill-xray", "other", 1), strings.Replace(cleanSARIF, "no-reported-gap", "unknown", 1), strings.Replace(cleanSARIF, `"coverage":"no-reported-gap"`, `"coverage":"no-reported-gap","llmUsage":{}`, 1)} {
		if _, err := parseSARIF([]byte(bad), &e); err == nil {
			t.Fatal("accepted invalid pinned interface")
		}
	}
}
func TestPerPackageSARIFProblemsDoNotStopSession(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	probes, scans := 0, 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			probes++
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		content := "malformed JSON"
		if scans == 2 {
			content = cleanSARIF
		}
		if e := os.WriteFile(args[2], []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	p := measuredFixture(t)
	first, _ := s.Scan(context.Background(), p)
	second, _ := s.Scan(context.Background(), p)
	if first.Status != "failed" || !Completed(second) || probes != 1 || scans != 2 {
		t.Fatal(first, second, probes, scans)
	}
}
func TestScannerConsoleBound(t *testing.T) {
	b := &cappedBuffer{remaining: 64 << 10}
	input := []byte(strings.Repeat("x", 128<<10))
	n, e := b.Write(input)
	if e != nil || n != len(input) || b.Len() != 64<<10 || !b.overflow {
		t.Fatal(n, e, b.Len())
	}
}

func TestReview2StagingFailureDoesNotDisableLaterPackage(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	good := measuredFixture(t)
	bad := good
	bad.Files = append([]File(nil), good.Files...)
	bad.Files[0].SHA256 = Sum([]byte("different"))
	first, _ := s.Scan(context.Background(), bad)
	second, _ := s.Scan(context.Background(), good)
	if first.Status != "failed" || !Completed(second) || scans != 1 {
		t.Fatal(first, second, scans)
	}
}

func TestReview2StagingWriteFailureDoesNotDisableLaterPackage(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	good := measuredFixture(t)
	long := strings.Repeat("x", 300)
	bad := Package{Path: good.Path, Complete: true, Files: []File{{Path: long, SHA256: Sum([]byte("data")), Size: 4}}, Data: map[string][]byte{long: []byte("data")}}
	first, _ := s.Scan(context.Background(), bad)
	second, _ := s.Scan(context.Background(), good)
	if first.Status != "failed" || !Completed(second) || scans != 1 {
		t.Fatal(first, second, scans)
	}
}

const review3FindingSARIF = `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"skill-xray","version":"0.1.0"}},"invocations":[{"executionSuccessful":true}],"properties":{"coverage":"no-reported-gap"},"results":[{"ruleId":"fixture-security","message":{"text":"fixture finding"},"properties":{"effectiveSeverity":"high","sxv":"SXV-fixture","coverage":"no-reported-gap"}}]}]}`

func TestReview3ScannerCoverageDecidesAfterExit(t *testing.T) {
	result := `[{"ruleId":"fixture-security","message":{"text":"fixture finding"},"properties":{"effectiveSeverity":"high","sxv":"SXV-fixture","coverage":"no-reported-gap"}}]`
	for _, tc := range []struct {
		name, report, status string
		code                 int
		err                  error
	}{
		{"complete findings exit 0", strings.Replace(cleanSARIF, `"results":[]`, `"results":`+result, 1), "completed_with_findings", 0, nil},
		{"complete findings exit 1", strings.Replace(cleanSARIF, `"results":[]`, `"results":`+result, 1), "completed_with_findings", 1, &exec.ExitError{}},
		{"complete empty exit 2", cleanSARIF, "completed_without_findings", 2, &exec.ExitError{}},
		{"partial exit 0", strings.Replace(cleanSARIF, "no-reported-gap", "incomplete", 1), "partial", 0, nil},
		{"partial exit 2", strings.Replace(cleanSARIF, "no-reported-gap", "incomplete", 1), "partial", 2, &exec.ExitError{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := scannerFixture(t)
			defer s.Close()
			scans := 0
			s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
				if args[0] == "version" {
					return "skill-xray 0.1.0", 0, nil
				}
				scans++
				if e := os.WriteFile(args[2], []byte(tc.report), 0600); e != nil {
					t.Fatal(e)
				}
				return "", tc.code, tc.err
			}
			for i := 0; i < 2; i++ {
				e, f := s.Scan(context.Background(), measuredFixture(t))
				if e.Status != tc.status {
					t.Fatal(e)
				}
				if strings.Contains(tc.name, "findings") && (len(f) != 1 || f[0].Kind != "security") {
					t.Fatal(f)
				}
				if tc.code != 0 && !strings.Contains(e.Message, fmt.Sprint(tc.code)) {
					t.Fatal("exit evidence lost", e)
				}
			}
			if scans != 2 {
				t.Fatal(scans)
			}
		})
	}
}

func TestReview3RealProcessNonzeroSARIF(t *testing.T) {
	source, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(tempDir(t), "scan-findings-exit1.exe")
	if e = os.WriteFile(exe, data, 0700); e != nil {
		t.Fatal(e)
	}
	s := NewScanner(exe, 10*time.Second, 2*time.Second)
	defer s.Close()
	for i := 0; i < 2; i++ {
		p := measuredFixture(t)
		evaluation, findings := s.Scan(context.Background(), p)
		if evaluation.Status != "completed_with_findings" || len(findings) != 1 || !strings.Contains(evaluation.Message, "scanner exit 1") {
			t.Fatal(evaluation, findings)
		}
		r := Report{Complete: Completed(evaluation), Skills: []Skill{{Package: p, Evaluations: []Evaluation{evaluation}, Findings: findings}}}
		if !r.Complete || Exit(r) != 1 {
			t.Fatal(r)
		}
	}
}

func TestReview4ScannerMessagesPreserveEveryCause(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		partial, measurement bool
		code                 int
		want                 []string
	}{
		{"partial exit 2", true, true, 2, []string{"scanner coverage incomplete", "scanner exit 2"}},
		{"partial exit 75", true, true, 75, []string{"scanner coverage incomplete", "scanner exit 75"}},
		{"clean scan incomplete measurement", false, false, 0, []string{"incomplete measurement"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := scannerFixture(t)
			defer s.Close()
			s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
				if args[0] == "version" {
					return "skill-xray 0.1.0", 0, nil
				}
				report := cleanSARIF
				if tc.partial {
					report = strings.Replace(report, "no-reported-gap", "incomplete", 1)
				}
				if e := os.WriteFile(args[2], []byte(report), 0600); e != nil {
					t.Fatal(e)
				}
				if tc.code != 0 {
					return "", tc.code, &exec.ExitError{}
				}
				return "", 0, nil
			}
			p := measuredFixture(t)
			p.Complete = tc.measurement
			e, _ := s.Scan(context.Background(), p)
			for _, want := range tc.want {
				if !strings.Contains(e.Message, want) {
					t.Fatal(e)
				}
			}
			if strings.HasSuffix(e.Message, "; ") {
				t.Fatal("dangling separator", e)
			}
		})
	}
}

func TestReviewX3ConsoleLimitIsPerPackage(t *testing.T) {
	src, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(src)
	if e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(tempDir(t), "scan-flood.exe")
	if e = os.WriteFile(exe, data, 0700); e != nil {
		t.Fatal(e)
	}
	p := measuredFixture(t)
	s := NewScanner(exe, 10*time.Second, 2*time.Second, p)
	defer s.Close()
	for i := 0; i < 2; i++ {
		evaluation, _ := s.Scan(context.Background(), p)
		if evaluation.Status != "partial" || !strings.Contains(evaluation.Message, "console output exceeded") || strings.Contains(evaluation.Message, "launch") {
			t.Fatal(evaluation)
		}
	}
	log, _ := os.ReadFile(exe + ".log")
	if strings.Count(string(log), "scan\n") != 2 || strings.Count(string(log), "version\n") != 1 {
		t.Fatal(string(log))
	}
}

func TestReviewScannerTempCannotBeInsideAnyPackage(t *testing.T) {
	p := measuredFixture(t)
	other := measuredFixture(t)
	base := filepath.Join(other.Path, "tmp")
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, base)
	}
	exe := fixture(t, tempDir(t), "trusted-executable", "data")
	s := NewScanner(exe, time.Second, time.Second, p, other)
	defer s.Close()
	launched := false
	s.process = func(context.Context, string, []string) (string, int, error) {
		launched = true
		return "skill-xray 0.1.0", 0, nil
	}
	evaluation, _ := s.Scan(context.Background(), p)
	if launched || evaluation.Status != "failed" || !strings.Contains(evaluation.Message, "inside audited package") {
		t.Fatal(launched, evaluation)
	}
	entries, e := os.ReadDir(base)
	if e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
}

func TestReviewX3ExitedProcessIsNotALaunchFailure(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", -1, &exec.ExitError{}
	}
	for i := 0; i < 2; i++ {
		evaluation, _ := s.Scan(context.Background(), measuredFixture(t))
		if evaluation.Status != "partial" {
			t.Fatal(evaluation)
		}
	}
	if scans != 2 {
		t.Fatal(scans)
	}
}

func TestX4DefaultProbeAllowsColdStart(t *testing.T) {
	exe := fixture(t, tempDir(t), "scanner", "trusted fake data")
	s := NewScanner(exe, time.Minute, 30*time.Second)
	defer s.Close()
	s.process = func(ctx context.Context, _ string, args []string) (string, int, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 6*time.Second {
			t.Fatal("cold probe would be capped below six seconds")
		}
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	e, _ := s.Scan(context.Background(), measuredFixture(t))
	if !Completed(e) {
		t.Fatal(e)
	}
}
