package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type InventoryBudget struct{ OutputBytes, DiagnosticBytes, Diagnostics int }

func defaultInventoryBudget() InventoryBudget { return InventoryBudget{StoreLimit, 1 << 20, 1000} }

type inventoryBudget struct {
	inv                                                  *Inventory
	limit                                                InventoryBudget
	used, reserve, diagnosticBytes, packages, exclusions int
}

func newInventoryBudget(inv *Inventory, override *InventoryBudget) *inventoryBudget {
	limits := defaultInventoryBudget()
	if override != nil {
		limits = *override
	}
	b, _ := Encode(*inv)
	s := &inventoryBudget{inv: inv, limit: limits, used: len(b), reserve: min(1<<20, limits.OutputBytes/4)}
	if s.reserve < 512 || s.used+s.reserve > limits.OutputBytes {
		inv.outputFailure = fmt.Errorf("inventory scope and required coverage headroom exceed output budget")
		inv.Complete = false
	}
	return s
}
func (s *inventoryBudget) partial() {
	if s.inv.Complete {
		s.used++
		s.inv.Complete = false
	}
}
func (s *inventoryBudget) entry() {
	old := len(strconv.Itoa(s.inv.Entries))
	s.inv.Entries++
	s.used += len(strconv.Itoa(s.inv.Entries)) - old
}
func (s *inventoryBudget) packageMeasurement(p Package) bool {
	n, e := arrayGrowth(p, s.packages, 2)
	if e != nil || s.used+n+s.reserve > s.limit.OutputBytes {
		return false
	}
	s.inv.Packages = append(s.inv.Packages, p)
	s.used += n
	s.packages++
	return true
}
func (s *inventoryBudget) omit(code string) {
	s.partial()
	if !s.inv.DiagnosticsTruncated {
		s.inv.DiagnosticsTruncated = true
		s.used += fieldPrefix("diagnostics_truncated") + len("true")
	}
	if s.inv.OmittedDiagnostics == nil {
		s.inv.OmittedDiagnostics = map[string]int{}
		s.used += fieldPrefix("omitted_diagnostics") + 2
	}
	old := s.inv.OmittedDiagnostics[code]
	if old == 0 {
		n, _ := mapItemGrowth(code, 1, len(s.inv.OmittedDiagnostics), 2)
		s.used += n
	} else {
		s.used += len(strconv.Itoa(old+1)) - len(strconv.Itoa(old))
	}
	s.inv.OmittedDiagnostics[code] = old + 1
}
func (s *inventoryBudget) gap(d Diagnostic) {
	if d.Code == "inventory_output_budget" && s.inv.BudgetStop == "" {
		s.inv.BudgetStop = d.Path
		b, _ := json.Marshal(d.Path)
		s.used += fieldPrefix("budget_stop") + len(b)
	}
	s.partial()
	n, e := arrayGrowth(d, len(s.inv.Diagnostics), 2)
	field := 0
	if len(s.inv.Diagnostics) == 0 {
		field = fieldPrefix("diagnostics") + 2
	}
	if e != nil || len(s.inv.Diagnostics) >= s.limit.Diagnostics || s.diagnosticBytes+n > s.limit.DiagnosticBytes || s.used+n+field+s.reserve > s.limit.OutputBytes {
		s.omit(d.Code)
		return
	}
	s.inv.Diagnostics = append(s.inv.Diagnostics, d)
	s.used += n + field
	s.diagnosticBytes += n
}

// Exclusions are few bounded rule summaries. Replace only the changed summary;
// never serialize the accumulated packages to account for a count/example.
func (s *inventoryBudget) exclusion(old *Exclusion, next Exclusion) bool {
	delta := 0
	if old == nil {
		delta, _ = arrayGrowth(next, s.exclusions, 2)
		if s.exclusions == 0 {
			delta += fieldPrefix("exclusions") + 2
		}
	} else {
		a, _ := formattedSize(*old, 2)
		b, _ := formattedSize(next, 2)
		delta = b - a
	}
	if s.used+delta+s.reserve > s.limit.OutputBytes {
		return false
	}
	s.used += delta
	if old == nil {
		s.exclusions++
	}
	return true
}
func (s *inventoryBudget) finish() {
	if s.inv.DiagnosticsTruncated {
		d := diag("inventory_diagnostics_truncated", "", "gap-location details were truncated; omitted_diagnostics gives exact counts by code; discovery remains partial")
		n, _ := arrayGrowth(d, len(s.inv.Diagnostics), 2)
		if len(s.inv.Diagnostics) == 0 {
			n += fieldPrefix("diagnostics") + 2
		}
		s.inv.Diagnostics = append(s.inv.Diagnostics, d)
		s.used += n
	}
	if s.inv.outputFailure == nil && s.used > s.limit.OutputBytes {
		s.partial()
		s.inv.outputFailure = fmt.Errorf("mandatory inventory coverage evidence exceeds output budget")
		s.inv.Diagnostics = append(s.inv.Diagnostics, diag("inventory_output_budget", s.inv.BudgetStop, s.inv.outputFailure.Error()))
	}
}
