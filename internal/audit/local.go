package audit

import (
	"bytes"
	"net/url"
	"path"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func literal(w *syntax.Word) (string, bool) {
	if w == nil {
		return "", false
	}
	var parts func([]syntax.WordPart) (string, bool)
	parts = func(ps []syntax.WordPart) (string, bool) {
		var b strings.Builder
		for _, p := range ps {
			switch x := p.(type) {
			case *syntax.Lit:
				if strings.Contains(x.Value, "\\") {
					return "", false
				}
				b.WriteString(x.Value)
			case *syntax.SglQuoted:
				if x.Dollar {
					return "", false
				}
				b.WriteString(x.Value)
			case *syntax.DblQuoted:
				if x.Dollar {
					return "", false
				}
				s, ok := parts(x.Parts)
				if !ok {
					return "", false
				}
				b.WriteString(s)
			default:
				return "", false
			}
		}
		return b.String(), true
	}
	return parts(w.Parts)
}
func argv(s *syntax.Stmt) []string {
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(c.Assigns) > 0 {
		return nil
	}
	var a []string
	for _, w := range c.Args {
		v, ok := literal(w)
		if !ok {
			return nil
		}
		a = append(a, v)
	}
	return a
}
func downloader(a []string) bool {
	if len(a) < 2 || (a[0] != "curl" && a[0] != "wget") {
		return false
	}
	urls := 0
	stdout := a[0] == "curl"
	terminated := false
	for i := 1; i < len(a); i++ {
		v := a[i]
		u, err := url.Parse(v)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			urls++
			continue
		}
		if v == "--" {
			terminated = true
			continue
		}
		if terminated {
			return false
		}
		if a[0] == "curl" {
			if v == "--fail" || v == "--silent" || v == "--show-error" || v == "--location" {
				continue
			}
			if len(v) > 1 && v[0] == '-' && !strings.HasPrefix(v, "--") && strings.Trim(v[1:], "fsSL") == "" {
				continue
			}
			return false
		}
		switch v {
		case "-q", "--quiet", "--no-verbose":
		case "-O", "--output-document":
			i++
			if i >= len(a) || a[i] != "-" {
				return false
			}
			stdout = true
		case "-O-", "-qO-", "--output-document=-":
			stdout = true
		default:
			return false
		}
	}
	return urls == 1 && stdout
}
func interpreter(a []string) bool {
	if len(a) == 0 || (a[0] != "sh" && a[0] != "bash") {
		return false
	}
	stdin, terminated := false, false
	for _, v := range a[1:] {
		if terminated {
			return stdin
		}
		if v == "--" || v == "-" {
			terminated = true
			continue
		}
		if !strings.HasPrefix(v, "-") {
			return stdin
		}
		if len(v) < 2 || strings.Trim(v[1:], "euxvs") != "" {
			return false
		}
		if strings.ContainsRune(v[1:], 's') {
			stdin = true
		}

	}
	return true
}
func pipePreserved(s *syntax.Stmt, fd int) bool {
	for _, r := range s.Redirs {
		n := 1
		switch r.Op {
		case syntax.RdrIn, syntax.RdrInOut, syntax.DplIn, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
			n = 0
		}
		if r.N != nil {
			var e error
			n, e = strconv.Atoi(r.N.Value)
			if e != nil {
				return false
			}
		}
		if r.Op == syntax.RdrAll || r.Op == syntax.AppAll {
			if fd == 1 {
				return false
			}
		}
		if r.Op == syntax.DplIn || r.Op == syntax.DplOut {
			v, ok := literal(r.Word)
			if !ok {
				return false
			}
			if v != "-" {
				if _, e := strconv.Atoi(v); e != nil {
					return false
				}
			}
		}
		if n == fd {
			return false
		}
	}
	return true
}
func shellEligible(file string, b []byte) (bool, string) {
	ext := path.Ext(file)
	if ext != ".sh" && ext != ".bash" {
		switch ext {
		case ".zsh", ".ksh", ".fish":
			return false, "unsupported shell suffix"
		}
		return false, ""
	}
	if !bytes.HasPrefix(b, []byte("#!")) {
		return true, ""
	}
	a := strings.Fields(strings.SplitN(string(b[2:]), "\n", 2)[0])
	if len(a) == 0 {
		return false, "unsupported shebang"
	}
	if path.Base(a[0]) == "env" {
		a = a[1:]
		if len(a) > 0 && a[0] == "-S" {
			a = a[1:]
		}
	}
	if len(a) == 0 || (path.Base(a[0]) != "sh" && path.Base(a[0]) != "bash") {
		return false, "unsupported interpreter"
	}
	for _, v := range a[1:] {
		if len(v) < 2 || (v[0] != '-' && v[0] != '+') || strings.Trim(v[1:], "abefhkmnptuvxBCEHPT") != "" {
			return false, "unsupported shebang arguments"
		}
	}
	return true, ""
}
func LocalScan(p Package) (Evaluation, []Finding) {
	e := Evaluation{Scanner: "skill-audit", Version: EvaluationVersion, Status: "completed_without_findings", Analyzed: []string{}, OutsideScope: []string{}}
	var findings []Finding
	if !p.Complete {
		e.Status = "partial"
		e.Message = "package measurement incomplete"
	}
	for _, f := range p.Files {
		b := p.Data[f.Path]
		ok, reason := shellEligible(f.Path, b)
		if !ok {
			if reason == "" || (path.Ext(f.Path) != ".sh" && path.Ext(f.Path) != ".bash") {
				e.OutsideScope = append(e.OutsideScope, f.Path)
			}
			if reason != "" {
				e.Status = "partial"
				e.Message = reason
			}
			continue
		}
		tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(b), f.Path)
		if err != nil {
			e.Status = "partial"
			e.Message = "shell parse error in " + f.Path
			continue
		}
		e.Analyzed = append(e.Analyzed, f.Path)
		seen := map[uint]bool{}
		syntax.Walk(tree, func(n syntax.Node) bool {
			bin, ok := n.(*syntax.BinaryCmd)
			if !ok || bin.Op != syntax.Pipe {
				return true
			}
			left, right := bin.X, bin.Y
			for {
				x, ok := left.Cmd.(*syntax.BinaryCmd)
				if !ok || (x.Op != syntax.Pipe && x.Op != syntax.PipeAll) || len(left.Redirs) > 0 {
					break
				}
				left = x.Y
			}
			for {
				x, ok := right.Cmd.(*syntax.BinaryCmd)
				if !ok || (x.Op != syntax.Pipe && x.Op != syntax.PipeAll) || len(right.Redirs) > 0 {
					break
				}
				right = x.X
			}
			if !downloader(argv(left)) || !interpreter(argv(right)) || !pipePreserved(left, 1) || !pipePreserved(right, 0) {
				return true
			}
			offset := left.Pos().Offset()
			if seen[offset] {
				return true
			}
			seen[offset] = true
			end := right.End().Offset()
			if int(end) > len(b) {
				return true
			}
			ev := b[offset:end]
			if len(ev) > 240 {
				ev = utf8Prefix(ev, 240)
				ev = append(append([]byte(nil), ev...), []byte("…")...)
			}
			findings = append(findings, Finding{Scanner: "skill-audit", ScannerVersion: EvaluationVersion, Kind: "security", RuleID: "SA001", RuleVersion: LocalRuleVersion, Severity: "high", File: f.Path, Line: int(left.Pos().Line()), Evidence: string(ev), Message: "Literal HTTP(S) download piped to a shell", Limitations: "Static syntax only; no execution, expansion, command resolution or general dataflow", Change: "not_compared"})
			return true
		})
	}
	if e.Status != "partial" && len(findings) > 0 {
		e.Status = "completed_with_findings"
	}
	return e, findings
}
