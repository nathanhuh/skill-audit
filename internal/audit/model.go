// Package audit implements bounded, on-demand observations. All package bytes are data.
package audit

import (
	"encoding/json"
	"time"
)

const (
	InventorySchema    = "skill-audit/inventory/v3"
	ReportSchema       = "skill-audit/report/v2"
	StateSchema        = "skill-audit/state/v3"
	DiscoveryPolicy    = "skill-audit/discovery/v3"
	CatalogSchema      = "skill-audit/catalog/v1"
	StoreSchema        = "skill-audit/catalog-store/v2"
	SearchSchema       = "skill-audit/search/v3"
	LegacySearchSchema = "skill-audit/search/v2"
	LocalRuleVersion   = "3"
	EvaluationVersion  = "SA001/v3;sh/v3.10.0;scope/v2"
	FileLimit          = 1 << 20
	PackageLimit       = 16 << 20
	PackageEntries     = 1000
	StoreLimit         = 64 << 20
	StateLimit         = 32 << 20
)

type Diagnostic struct {
	Scope      string `json:"scope,omitempty"`
	Path       string `json:"path"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Incomplete bool   `json:"incomplete"`
}
type Exclusion struct {
	Rule            string   `json:"rule"`
	Count           int      `json:"count"`
	Examples        []string `json:"examples"`
	OmittedExamples int      `json:"omitted_examples"`
}
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Package struct {
	Scope       string            `json:"scope"`
	Path        string            `json:"path"`
	Agent       string            `json:"agent"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Digest      string            `json:"digest,omitempty"`
	Complete    bool              `json:"complete"`
	Files       []File            `json:"files"`
	Diagnostics []Diagnostic      `json:"diagnostics,omitempty"`
	Data        map[string][]byte `json:"-"`
}
type Limits struct {
	Entries  int `json:"entries"`
	Packages int `json:"packages"`
	Depth    int `json:"depth"`
}
type Inventory struct {
	outputFailure        error
	BudgetStop           string         `json:"budget_stop,omitempty"`
	DiagnosticsTruncated bool           `json:"diagnostics_truncated,omitempty"`
	OmittedDiagnostics   map[string]int `json:"omitted_diagnostics,omitempty"`
	Limits               Limits         `json:"limits"`
	Schema               string         `json:"schema"`
	CreatedAt            time.Time      `json:"created_at"`
	Roots                []string       `json:"roots"`
	Home                 string         `json:"home,omitempty"`
	Policy               string         `json:"policy"`
	Complete             bool           `json:"complete"`
	Entries              int            `json:"entries"`
	Packages             []Package      `json:"packages"`
	Exclusions           []Exclusion    `json:"exclusions,omitempty"`
	Diagnostics          []Diagnostic   `json:"diagnostics,omitempty"`
	StatePath            string         `json:"state_path,omitempty"`
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
type Attribution struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Path       string `json:"path"`
	Data       []byte `json:"content"`
}
type Catalog struct {
	Schema      string        `json:"schema"`
	Skills      []Reference   `json:"skills"`
	Attribution []Attribution `json:"attribution,omitempty"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	Complete    bool          `json:"complete"`
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
	ScannerVersion string          `json:"scanner_version"`
	Kind           string          `json:"kind"`
	Scanner        string          `json:"scanner"`
	RuleID         string          `json:"rule_id"`
	RuleVersion    string          `json:"rule_version,omitempty"`
	Severity       string          `json:"severity"`
	File           string          `json:"path,omitempty"`
	Line           int             `json:"line"`
	Evidence       string          `json:"evidence,omitempty"`
	Message        string          `json:"explanation"`
	Limitations    string          `json:"limitations,omitempty"`
	Change         string          `json:"change,omitempty"`
	Original       json.RawMessage `json:"source,omitempty"`
}
type Evaluation struct {
	Analyzed     []string        `json:"analyzed"`
	OutsideScope []string        `json:"outside_scope"`
	Source       json.RawMessage `json:"source,omitempty"`
	Scanner      string          `json:"scanner"`
	Version      string          `json:"version"`
	Status       string          `json:"status"`
	Message      string          `json:"message,omitempty"`
}
type Lineage struct {
	Status         string       `json:"status"`
	Added          []string     `json:"added,omitempty"`
	Removed        []string     `json:"removed,omitempty"`
	Modified       []string     `json:"modified,omitempty"`
	BaselineOrigin *Correlation `json:"baseline_origin,omitempty"`
}
type Skill struct {
	Package
	Origins     []Candidate  `json:"origins"`
	Correlation Correlation  `json:"correlation"`
	Evaluations []Evaluation `json:"evaluations"`
	Findings    []Finding    `json:"findings"`
	Lineage     Lineage      `json:"-"`
}
type Recording struct {
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}
type Report struct {
	interrupted          bool
	BudgetStop           string
	DiagnosticsTruncated bool
	OmittedDiagnostics   map[string]int
	StatePath            string        `json:"-"`
	Limits               Limits        `json:"-"`
	Home                 string        `json:"-"`
	Missing              []WireLineage `json:"-"`
	Schema               string        `json:"schema"`
	CreatedAt            time.Time     `json:"created_at"`
	CatalogID            string        `json:"catalog_id"`
	Roots                []string      `json:"roots"`
	Policy               string        `json:"discovery_policy"`
	DiscoveryComplete    bool          `json:"discovery_complete"`
	Complete             bool          `json:"complete"`
	Exclusions           []Exclusion   `json:"exclusions,omitempty"`
	Diagnostics          []Diagnostic  `json:"diagnostics,omitempty"`
	Skills               []Skill       `json:"skills"`
	Recording            *Recording    `json:"recording,omitempty"`
}
type State struct {
	RawObservations []json.RawMessage `json:"-"`
	Schema          string            `json:"schema"`
	Observations    []Report          `json:"observations"`
}

func diag(code, path, message string) Diagnostic {
	return Diagnostic{Code: code, Path: path, Message: message, Incomplete: true}
}
func Identity(p Package) string { return p.Scope + "\x00" + p.Path }
func Completed(e Evaluation) bool {
	return e.Status == "completed_without_findings" || e.Status == "completed_with_findings"
}
func Exit(r Report) int {
	for _, s := range r.Skills {
		for _, f := range s.Findings {
			if f.Kind == "security" {
				return 1
			}
		}
	}
	if r.Recording != nil && r.Recording.Status == "not_recorded" {
		return 3
	}
	if !r.Complete || r.Recording != nil && r.Recording.Status == "committed_durability_unconfirmed" {
		return 2
	}
	return 0
}

func info(code, path, message string) Diagnostic {
	d := diag(code, path, message)
	d.Incomplete = false
	return d
}

// OutputError is a runtime admission failure, never an imported diagnostic.
func (i Inventory) OutputError() error { return i.outputFailure }
