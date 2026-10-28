package audit

import (
	"fmt"
	"io"
)

func Print(w io.Writer, r Report, exitCode int) {
	fmt.Fprintf(w, "skill-audit report/v1 | %d packages | catalog %s\n", len(r.Skills), r.CatalogVersion)
	fmt.Fprintln(w, "Inventory is not evidence that an agent loads a skill. Findings are risky patterns, not malicious-intent verdicts.")
	for _, root := range r.Roots {
		fmt.Fprintf(w, "root %s %q: %s\n", root.Scope, root.Path, root.Status)
	}
	diagnostic := func(d Diagnostic) {
		fmt.Fprintf(w, "  diagnostic %s %q: %q (incomplete=%t)\n", d.Code, d.Path, d.Message, d.Incomplete)
	}
	for _, s := range r.Skills {
		fmt.Fprintf(w, "\n[%s] %q %q\n  correlation=%s complete=%t files=%d digest=%s\n", s.Scope, s.Name, s.Path, s.Correlation.Status, s.Complete, len(s.Files), s.Digest)
		for _, c := range s.Correlation.Candidates {
			fmt.Fprintf(w, "  candidate %q: %s score=%.3f shared=%d union=%d\n", c.Reference.ID, c.Basis, c.Score, c.Shared, c.Union)
		}
		for _, e := range s.Evaluations {
			if e.Scanner == "skill-audit" {
				fmt.Fprintf(w, "  scanner %s %s: %s (analyzed=%d outside_scope=%d) %q\n", e.Scanner, e.Version, e.Status, len(e.Analyzed), len(e.OutsideScope), e.Message)
			} else {
				fmt.Fprintf(w, "  scanner %s %s: %s %q\n", e.Scanner, e.Version, e.Status, e.Message)
			}
		}
		for _, f := range s.Findings {
			fmt.Fprintf(w, "  %s %s:%q %q:%d [%s] %s\n    evidence=%q\n    explanation=%q\n", f.Severity, f.Scanner, f.RuleID, f.Path, f.Line, f.Kind, f.Change, f.Evidence, f.Explanation)
		}
		for _, d := range s.Diagnostics {
			diagnostic(d)
		}
	}
	for _, l := range r.Lineage {
		fmt.Fprintf(w, "\nlineage [%s] %q: %s\n", l.Scope, l.Path, l.Status)
		for _, o := range l.Origin {
			fmt.Fprintf(w, "  baseline origin %q (%q)\n", o.Reference.ID, o.Basis)
		}
		for _, c := range l.Changes {
			fmt.Fprintf(w, "  %s %q\n", c.Kind, c.Path)
		}
	}
	for _, d := range r.Diagnostics {
		diagnostic(d)
	}
	fmt.Fprintf(w, "\nexit=%d (0=no findings in selected scope; 1=findings; 2=operational/incomplete; 3=state not recorded)\n", exitCode)
}
