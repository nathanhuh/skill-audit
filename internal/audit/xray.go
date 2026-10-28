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
	"strings"
	"time"
)

const XRayVersion = "0.1.0"
const XRayCommit = "c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2"
const MaxScannerOutput = 64 << 10
const MaxSARIF = 8 << 20

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.overflow = true
		remaining := b.limit - b.Len()
		if remaining > 0 {
			b.Buffer.Write(p[:remaining])
		}
		return remaining, fmt.Errorf("scanner output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func runProcess(ctx context.Context, exe, dir string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	// The pinned CLI enables LLMs only via flags; additionally omit credentials and scanner env configuration.
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "XDG_CACHE_HOME", "LOCALAPPDATA", "SystemRoot"} {
		if v, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+v)
		}
	}
	out, errout := &boundedBuffer{limit: MaxScannerOutput}, &boundedBuffer{limit: MaxScannerOutput}
	cmd.Stdout = out
	cmd.Stderr = errout
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if out.overflow || errout.overflow {
		err = fmt.Errorf("scanner console output exceeded %d bytes", MaxScannerOutput)
		code = -1
	}
	return out.String(), errout.String(), code, err
}
func XRayScan(ctx context.Context, s *Skill, exe string, timeout time.Duration) {
	e := Evaluation{Scanner: "skill-xray", Status: "disabled", Analyzed: []string{}, OutsideScope: []string{}}
	defer func() { s.Evaluations = append(s.Evaluations, e); sortFindings(s.Findings) }()
	if exe == "" {
		return
	}
	e.Status = "unavailable"
	if !filepath.IsAbs(exe) {
		e.Message = "--xray must be an absolute path to a trusted executable"
		return
	}
	info, err := os.Stat(exe)
	if err != nil || !info.Mode().IsRegular() {
		e.Message = "scanner executable unavailable"
		return
	}
	e.Status = "failed"
	dir, err := os.MkdirTemp("", "skill-audit-xray-")
	if err != nil {
		e.Message = err.Error()
		return
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		e.Message = err.Error()
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stdout, stderr, _, err := runProcess(ctx, exe, dir, "version")
	if err != nil {
		e.Message = fmt.Sprintf("version probe: %v; %s", err, stderr)
		return
	}
	if strings.TrimSpace(stdout) != "skill-xray "+XRayVersion {
		e.Message = "unsupported scanner version; expected skill-xray " + XRayVersion + ", got " + strings.TrimSpace(stdout)
		return
	}
	e.Version = XRayVersion
	stage := filepath.Join(dir, "package")
	if err = os.Mkdir(stage, 0700); err != nil {
		e.Message = err.Error()
		return
	}
	for _, f := range s.Files {
		p := filepath.Join(stage, filepath.FromSlash(f.Path))
		if err = os.MkdirAll(filepath.Dir(p), 0700); err == nil {
			err = os.WriteFile(p, s.Data[f.Path], 0600)
		}
		if err != nil {
			e.Message = err.Error()
			return
		}
	}
	reportPath := filepath.Join(dir, "report.sarif.json")
	_, stderr, code, runErr := runProcess(ctx, exe, dir, "scan", "--output", reportPath, "--llm=false", "--", stage)
	if ctx.Err() != nil {
		e.Message = "scan canceled/timed out: " + ctx.Err().Error()
		return
	}
	if runErr != nil && code != 2 {
		e.Message = fmt.Sprintf("scanner failed: %v; %s", runErr, stderr)
		return
	}
	b, err := ReadBounded(reportPath, MaxSARIF)
	if err != nil {
		e.Message = fmt.Sprintf("missing/unreadable SARIF: %v; %s", err, stderr)
		return
	}
	findings, source, partial, err := parseSARIF(b)
	if err != nil {
		e.Message = "invalid SARIF: " + err.Error()
		return
	}
	s.Findings = append(s.Findings, findings...)
	e.Source = source
	count := 0
	for _, f := range findings {
		if f.Kind == "security" {
			count++
		}
	}
	e.Status = finishStatus(count, partial || !s.Complete || code != 0)
	e.Message = strings.TrimSpace(stderr)
	if e.Status == "partial" && e.Message == "" {
		e.Message = "scanner reports incomplete coverage or staged inventory was incomplete"
	}
}

type sarifResult struct {
	RuleID  string `json:"ruleId"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	Properties struct {
		Severity string          `json:"effectiveSeverity"`
		Vector   string          `json:"sxv"`
		Evidence json.RawMessage `json:"evidence"`
		Coverage string          `json:"coverage"`
	} `json:"properties"`
	Locations []struct {
		Physical struct {
			Artifact struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				Line int `json:"startLine"`
			} `json:"region"`
		} `json:"physicalLocation"`
	} `json:"locations"`
}

func parseSARIF(b []byte) ([]Finding, json.RawMessage, bool, error) {
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"driver"`
			} `json:"tool"`
			Results     []json.RawMessage `json:"results"`
			Invocations []struct {
				Success *bool `json:"executionSuccessful"`
			} `json:"invocations"`
			Properties json.RawMessage `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil, false, err
	}
	bad := func(msg string) ([]Finding, json.RawMessage, bool, error) { return nil, nil, false, errors.New(msg) }
	if doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		return bad("expected SARIF 2.1.0 with exactly one run")
	}
	r := doc.Runs[0]
	if r.Tool.Driver.Name != "skill-xray" || r.Tool.Driver.Version != XRayVersion {
		return bad("unexpected tool/version")
	}
	var props struct {
		Coverage string            `json:"coverage"`
		LLM      json.RawMessage   `json:"llmUsage"`
		Errors   []json.RawMessage `json:"contextErrors"`
	}
	if err := json.Unmarshal(r.Properties, &props); err != nil {
		return bad("missing run properties")
	}
	if props.Coverage != "no-reported-gap" && props.Coverage != "incomplete" {
		return bad("missing/unknown coverage status")
	}
	if len(props.LLM) > 0 && string(props.LLM) != "null" {
		return bad("unexpected LLM lane in report")
	}
	if r.Results == nil || len(r.Invocations) != 1 || r.Invocations[0].Success == nil {
		return bad("missing results or execution status")
	}
	partial := props.Coverage == "incomplete" || !*r.Invocations[0].Success || len(props.Errors) > 0
	findings := []Finding{}
	for _, raw := range r.Results {
		var v sarifResult
		if err := json.Unmarshal(raw, &v); err != nil {
			return bad("malformed result")
		}
		if v.RuleID == "" || v.Message.Text == "" {
			return bad("result missing rule/message")
		}
		switch v.Properties.Severity {
		case "critical", "high", "medium", "low", "info":
		default:
			return bad("result missing/unknown original severity")
		}
		if v.Properties.Coverage != "no-reported-gap" && v.Properties.Coverage != "incomplete" {
			return bad("result missing/unknown coverage")
		}
		if v.Properties.Coverage == "incomplete" {
			partial = true
		}
		kind := "coverage"
		if v.Properties.Vector != "" {
			kind = "security"
		}
		f := Finding{Scanner: "skill-xray", ScannerVersion: XRayVersion, RuleID: v.RuleID, Severity: v.Properties.Severity, Kind: kind, Evidence: string(v.Properties.Evidence), Explanation: v.Message.Text, Limitations: "Source-specific evidence, limitations, severity and disposition are preserved in source; no equivalence with SA001 is implied.", Source: raw}
		if len(v.Locations) > 0 {
			loc := v.Locations[0].Physical
			u, err := url.PathUnescape(loc.Artifact.URI)
			if err != nil || u == "" || strings.Contains(u, "\\") || strings.HasPrefix(u, "/") || strings.Contains(u, ":") || u == ".." || strings.HasPrefix(u, "../") || strings.Contains(u, "/../") {
				return bad("invalid relative result location")
			}
			f.Path = u
			f.Line = loc.Region.Line
		}
		findings = append(findings, f)
	}
	sortFindings(findings)
	return findings, r.Properties, partial, nil
}
