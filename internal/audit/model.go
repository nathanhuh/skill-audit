// Package audit inventories untrusted skill data without executing it.
package audit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const ReportSchema = "skill-audit/report/v1"
const LocalRuleVersion = "2"
const LocalVersion = "SA001/v" + LocalRuleVersion + ";sh/v3.10.0;scope/v2"

type Diagnostic struct {
	Scope      string `json:"scope,omitempty"`
	Path       string `json:"path"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Incomplete bool   `json:"incomplete"`
}
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Root struct {
	Scope    string `json:"scope"`
	Path     string `json:"path"`
	Explicit bool   `json:"explicit"`
	Status   string `json:"status"`
}
type Reference struct {
	ID           string            `json:"id"`
	Repository   string            `json:"repository"`
	Commit       string            `json:"commit"`
	UpstreamPath string            `json:"upstream_path"`
	Digest       string            `json:"digest"`
	Files        []File            `json:"files"`
	Data         map[string][]byte `json:"-"`
}
type Candidate struct {
	Reference Reference `json:"reference"`
	Basis     string    `json:"basis"`
	Score     float64   `json:"score"`
	Shared    int       `json:"shared_lines"`
	Union     int       `json:"union_lines"`
}
type Correlation struct {
	Status     string      `json:"status"`
	Method     string      `json:"method"`
	Candidates []Candidate `json:"candidates"`
}
type Finding struct {
	Scanner        string          `json:"scanner"`
	ScannerVersion string          `json:"scanner_version"`
	RuleID         string          `json:"rule_id"`
	RuleVersion    string          `json:"rule_version,omitempty"`
	Severity       string          `json:"severity"`
	Kind           string          `json:"kind"`
	Path           string          `json:"path,omitempty"`
	Line           int             `json:"line,omitempty"`
	Evidence       string          `json:"evidence"`
	Explanation    string          `json:"explanation"`
	Limitations    string          `json:"limitations"`
	Change         string          `json:"change,omitempty"`
	Source         json.RawMessage `json:"source,omitempty"`
}
type Evaluation struct {
	Scanner      string          `json:"scanner"`
	Version      string          `json:"version,omitempty"`
	Status       string          `json:"status"`
	Analyzed     []string        `json:"analyzed"`
	OutsideScope []string        `json:"outside_scope"`
	Message      string          `json:"message,omitempty"`
	Source       json.RawMessage `json:"source,omitempty"`
}
type Skill struct {
	Scope       string            `json:"scope"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Digest      string            `json:"digest,omitempty"`
	Complete    bool              `json:"complete"`
	Files       []File            `json:"files"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
	Correlation Correlation       `json:"correlation"`
	Origins     []Candidate       `json:"origins"`
	Evaluations []Evaluation      `json:"evaluations"`
	Findings    []Finding         `json:"findings"`
	Data        map[string][]byte `json:"-"`
}
type Change struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type Lineage struct {
	Scope   string      `json:"scope"`
	Path    string      `json:"path"`
	Status  string      `json:"status"`
	Origin  []Candidate `json:"origin"`
	Changes []Change    `json:"changes"`
}
type Report struct {
	Schema         string       `json:"schema"`
	CatalogVersion string       `json:"catalog_version"`
	Roots          []Root       `json:"roots"`
	Skills         []Skill      `json:"skills"`
	Lineage        []Lineage    `json:"lineage"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
}

// Hash fingerprints raw bytes without normalization.
func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Digest(files []File) string {
	fs := append([]File(nil), files...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
	h := sha256.New()
	h.Write([]byte("skill-audit/directory/v1\x00"))
	for _, f := range fs {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(f.Path)))
		h.Write(n[:])
		h.Write([]byte(f.Path))
		b, _ := hex.DecodeString(f.SHA256)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func (s Skill) identity() string { return s.Scope + "\x00" + s.Path }
func completed(s string) bool {
	return s == "completed_with_findings" || s == "completed_without_findings"
}
func finishStatus(findings int, partial bool) string {
	if partial {
		return "partial"
	}
	if findings > 0 {
		return "completed_with_findings"
	}
	return "completed_without_findings"
}

// HasCoverageGaps is the single completeness policy for reports and baselines.
// Diagnostic.Incomplete records whether an observation is an operational gap;
// explicit policy exclusions remain visible without becoming failures.
func (r Report) HasCoverageGaps() bool {
	for _, d := range r.Diagnostics {
		if d.Incomplete {
			return true
		}
	}
	for _, s := range r.Skills {
		if !s.Complete {
			return true
		}
		for _, d := range s.Diagnostics {
			if d.Incomplete {
				return true
			}
		}
		for _, e := range s.Evaluations {
			if e.Status != "disabled" && !completed(e.Status) {
				return true
			}
		}
	}
	return false
}

// ExitCode describes analysis only; the CLI owns persistence-operation status.
func (r Report) ExitCode() int {
	if r.HasCoverageGaps() {
		return 2
	}
	for _, s := range r.Skills {
		for _, f := range s.Findings {
			if f.Kind == "security" {
				return 1
			}
		}
	}
	return 0
}
func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Scanner != b.Scanner {
			return a.Scanner < b.Scanner
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Evidence < b.Evidence
	})
}
