package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Skill X-Ray is a separately attributed, user-supplied scanner.
const XrayVersion = "0.1.0"
const XrayCommit = "c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2"

type Scanner struct {
	executable       string
	timeout, reserve time.Duration
	terminal         *Evaluation
	probed           bool
	temp             string
	packages         []Package
	process          func(context.Context, string, []string) (string, int, error)
}

func NewScanner(executable string, operation, phase time.Duration, packages ...Package) *Scanner {
	s := &Scanner{executable: executable, timeout: phase, reserve: operation / 4, packages: append([]Package(nil), packages...)}
	s.process = func(ctx context.Context, exe string, args []string) (string, int, error) {
		return runScanner(ctx, exe, args, s.temp)
	}
	if phase <= 0 {
		s.timeout = 30 * time.Second
	}
	if executable == "" {
		e := Evaluation{Scanner: "skill-xray", Version: XrayVersion, Status: "disabled"}
		s.terminal = &e
		return s
	}
	fail := func(msg string) {
		e := Evaluation{Scanner: "skill-xray", Version: XrayVersion, Status: "unavailable", Message: msg}
		s.terminal = &e
	}
	if !filepath.IsAbs(executable) {
		fail("scanner path must be absolute")
		return s
	}
	i, e := os.Stat(executable)
	if e != nil || !i.Mode().IsRegular() {
		fail("scanner path must name an existing regular file")
	}
	return s
}
func (s *Scanner) Close() {
	if s.temp != "" {
		os.RemoveAll(s.temp)
	}
}
func (s *Scanner) phase(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	limit := s.timeout
	if deadline, ok := ctx.Deadline(); ok {
		left := time.Until(deadline) - s.reserve
		if left <= 0 {
			return nil, nil, fmt.Errorf("operation budget reserved for local measurement")
		}
		if left < limit {
			limit = left
		}
	}
	phase, cancel := context.WithTimeout(ctx, limit)
	return phase, cancel, nil
}

type cappedBuffer struct {
	buffer    bytes.Buffer
	remaining int
	overflow  bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > b.remaining {
		b.overflow = true
		p = p[:b.remaining]
	}
	b.remaining -= len(p)
	b.buffer.Write(p)
	return n, nil
}

func (b *cappedBuffer) String() string { return b.buffer.String() }
func (b *cappedBuffer) Len() int       { return b.buffer.Len() }

type consoleLimitError struct{}

func (consoleLimitError) Error() string {
	return "scanner console output exceeded 64 KiB; logs truncated"
}
func runScanner(ctx context.Context, exe string, args []string, dir string) (string, int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = scannerEnv()
	cmd.Dir = dir
	if len(args) > 2 && args[0] == "scan" {
		cmd.Dir = filepath.Dir(args[2])
	}
	configureProcess(cmd)
	out := &cappedBuffer{remaining: 64 << 10}
	cmd.Stdout = out
	stderr := &cappedBuffer{remaining: 64 << 10}
	cmd.Stderr = stderr
	e := cmd.Run()
	if ctx.Err() != nil {
		return out.String(), -1, ctx.Err()
	}
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if out.overflow || stderr.overflow {
		return out.String(), code, consoleLimitError{}
	}
	return out.String(), code, e
}
func scannerEnv() []string {
	env := []string{}
	for _, k := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LOCALAPPDATA"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}
func (s *Scanner) fail(message string) Evaluation {
	e := Evaluation{Scanner: "skill-xray", Version: XrayVersion, Status: "failed", Message: message}
	s.terminal = &e
	return e
}
func (s *Scanner) Scan(ctx context.Context, p Package) (Evaluation, []Finding) {
	if s.terminal != nil {
		return *s.terminal, nil
	}
	if !s.probed {
		s.packages = append(s.packages, Package{Path: p.Path})
		phase, cancel, e := s.phase(ctx)
		if e != nil {
			return s.fail(e.Error()), nil
		}
		if err := s.prepareTemp(); err != nil {
			cancel()
			return s.fail(err.Error()), nil
		}
		output, code, err := s.process(phase, s.executable, []string{"version"})
		var overflow consoleLimitError
		if code == 75 && phase.Err() == nil && !errors.As(err, &overflow) {
			output, _, err = s.process(phase, s.executable, []string{"version"})
		}
		cancel()
		if err != nil {
			return s.fail("version probe failed: " + safeProcessError(err)), nil
		}
		if strings.TrimSpace(output) != "skill-xray "+XrayVersion {
			return s.fail("unsupported scanner version"), nil
		}
		s.probed = true
	}
	phase, cancel, e := s.phase(ctx)
	if e != nil {
		return s.fail(e.Error()), nil
	}
	defer cancel()
	dir, e := os.MkdirTemp(s.temp, "scan-*")
	if e != nil {
		return packageScannerFailure("scanner staging failed"), nil
	}
	defer os.RemoveAll(dir)
	staged := filepath.Join(dir, "package")
	if e = os.Mkdir(staged, 0700); e != nil {
		return packageScannerFailure("scanner staging failed"), nil
	}
	for _, f := range p.Files {
		if !SafeRelative(f.Path) {
			return packageScannerFailure("unsafe staged path"), nil
		}
		b := p.Data[f.Path]
		if Sum(b) != f.SHA256 {
			return packageScannerFailure("staged content hash mismatch"), nil
		}
		dest := filepath.Join(staged, filepath.FromSlash(f.Path))
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e == nil {
			e = os.WriteFile(dest, b, 0600)
		}
		if e != nil {
			return packageScannerFailure("scanner staging failed"), nil
		}
	}
	reportPath := filepath.Join(dir, "report.sarif")
	_, code, err := s.process(phase, s.executable, []string{"scan", "--output", reportPath, "--llm=false", "--", staged})
	if phase.Err() != nil {
		return s.fail("scan timeout or cancellation: " + phase.Err().Error()), nil
	}
	var exited *exec.ExitError
	var console consoleLimitError
	if code < 0 && err != nil && !errors.As(err, &exited) && !errors.As(err, &console) {
		return s.fail("scan launch failed: " + safeProcessError(err)), nil
	}
	evaluation := Evaluation{Scanner: "skill-xray", Version: XrayVersion, Status: "failed"}
	b, e := ReadBounded(reportPath, 8<<20)
	if e != nil {
		evaluation.Message = "scanner report unavailable"
		if err != nil {
			evaluation.Message += "; " + safeProcessError(err)
		}
		return evaluation, nil
	}
	findings, e := parseSARIF(b, &evaluation)
	if e != nil {
		evaluation.Status = "failed"
		evaluation.Message = e.Error()
		return evaluation, nil
	}
	if !p.Complete || code != 0 || err != nil {
		if Completed(evaluation) {
			evaluation.Status = "partial"
		}
		evaluation.Message = fmt.Sprintf("incomplete measurement or scanner exit %d", code)
		if err != nil {
			evaluation.Message = safeProcessError(err)
		}
	}
	return evaluation, findings
}
func safeProcessError(e error) string {
	var overflow consoleLimitError
	if errors.As(e, &overflow) {
		return overflow.Error()
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "deadline exceeded"
	}
	if errors.Is(e, context.Canceled) {
		return "canceled"
	}
	var ee *exec.ExitError
	if errors.As(e, &ee) {
		return fmt.Sprintf("exit %d", ee.ExitCode())
	}
	return "scanner process error"
}
func parseSARIF(b []byte, e *Evaluation) ([]Finding, error) {
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct{ Name, Version string } `json:"driver"`
			} `json:"tool"`
			Properties  json.RawMessage `json:"properties"`
			Invocations []struct {
				Success *bool `json:"executionSuccessful"`
			} `json:"invocations"`
			Results []json.RawMessage `json:"results"`
		} `json:"runs"`
	}
	bad := func(msg string) ([]Finding, error) { return nil, fmt.Errorf("invalid SARIF: %s", msg) }
	if json.Unmarshal(b, &doc) != nil || doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		return bad("expected 2.1.0 with one run")
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Name != "skill-xray" || run.Tool.Driver.Version != XrayVersion {
		return bad("unexpected tool or version")
	}
	var props struct {
		Coverage string            `json:"coverage"`
		LLM      json.RawMessage   `json:"llmUsage"`
		Errors   []json.RawMessage `json:"contextErrors"`
	}
	if json.Unmarshal(run.Properties, &props) != nil || (props.Coverage != "no-reported-gap" && props.Coverage != "incomplete") || len(props.LLM) > 0 && string(props.LLM) != "null" || len(run.Invocations) != 1 || run.Invocations[0].Success == nil || run.Results == nil {
		return bad("missing coverage, results, execution status or unexpected LLM lane")
	}
	partial := props.Coverage == "incomplete" || !*run.Invocations[0].Success || len(props.Errors) > 0
	e.Source = run.Properties
	findings := []Finding{}
	for _, raw := range run.Results {
		var result struct {
			RuleID     string                `json:"ruleId"`
			Message    struct{ Text string } `json:"message"`
			Properties struct {
				Severity string          `json:"effectiveSeverity"`
				Vector   string          `json:"sxv"`
				Evidence json.RawMessage `json:"evidence"`
				Coverage string          `json:"coverage"`
			} `json:"properties"`
			Locations []struct {
				Physical struct {
					Artifact struct{ URI string } `json:"artifactLocation"`
					Region   struct {
						Line int `json:"startLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		}
		if json.Unmarshal(raw, &result) != nil || result.RuleID == "" || result.Message.Text == "" {
			return bad("missing result rule or message")
		}
		switch result.Properties.Severity {
		case "critical", "high", "medium", "low", "info":
		default:
			return bad("missing original severity")
		}
		switch result.Properties.Coverage {
		case "incomplete":
			partial = true
		case "no-reported-gap":
		default:
			return bad("missing result coverage")
		}
		kind := "coverage"
		if result.Properties.Vector != "" {
			kind = "security"
		}
		f := Finding{Scanner: "skill-xray", ScannerVersion: XrayVersion, RuleID: result.RuleID, Kind: kind, Severity: result.Properties.Severity, Message: result.Message.Text, Evidence: string(result.Properties.Evidence), Original: raw, Change: "not_compared", Limitations: "Original scanner evidence, severity and limitations are preserved in source"}
		if len(result.Locations) > 0 {
			l := result.Locations[0].Physical
			relative, err := url.PathUnescape(l.Artifact.URI)
			if err != nil || !SafeRelative(relative) {
				return bad("unsafe result location")
			}
			f.File = relative
			f.Line = l.Region.Line
		}
		findings = append(findings, f)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.RuleID < b.RuleID
	})
	if partial {
		e.Status = "partial"
		e.Message = "scanner coverage incomplete"
	} else {
		count := 0
		for _, f := range findings {
			if f.Kind == "security" {
				count++
			}
		}
		e.Status = "completed_without_findings"
		if count > 0 {
			e.Status = "completed_with_findings"
		}
	}
	return findings, nil
}
func resolvedTemp() string {
	p, e := filepath.EvalSymlinks(os.TempDir())
	if e != nil {
		return os.TempDir()
	}
	return p
}

// Validate the real temp base against every audited package before any write.
func (s *Scanner) prepareTemp() error {
	if s.temp != "" {
		return nil
	}
	base := resolvedTemp()
	if e := NoLinks(base); e != nil {
		return fmt.Errorf("scanner staging refused: %w", e)
	}
	for _, p := range s.packages {
		if p.Path == "" {
			continue
		}
		inside := Within(p.Path, base)
		pkg, err := os.Lstat(p.Path)
		if err == nil {
			for q := base; ; q = filepath.Dir(q) {
				info, e := os.Lstat(q)
				if e == nil && os.SameFile(pkg, info) {
					inside = true
					break
				}
				if filepath.Dir(q) == q {
					break
				}
			}
		}
		if inside {
			return fmt.Errorf("scanner staging refused: temporary directory is inside audited package %q", p.Path)
		}
	}
	if e := OutputSafe(filepath.Join(base, ".skill-audit-stage-check"), nil, s.packages); e != nil {
		return fmt.Errorf("scanner staging refused: %w", e)
	}
	var e error
	s.temp, e = os.MkdirTemp(base, "skill-audit-scanner-*")
	if e != nil {
		return fmt.Errorf("scanner staging failed")
	}
	return nil
}

func packageScannerFailure(message string) Evaluation {
	return Evaluation{Scanner: "skill-xray", Version: XrayVersion, Status: "failed", Message: message}
}
