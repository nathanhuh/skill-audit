package audit

import (
	"bytes"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"mvdan.cc/sh/v3/syntax"
)

const localLimitations = "Adjacent literal curl/wget stdout to sh/bash stdin stages connected by | in .sh/.bash; conservative options. Redirects must leave that connection intact. No descriptor moves, variable descriptors, expansion, alias/function resolution, general dataflow, Markdown fences, or reachability analysis. A pattern is not proof of malicious intent."

func LocalScan(s *Skill) {
	initialFindings := len(s.Findings)
	e := Evaluation{Scanner: "skill-audit", Version: LocalVersion, Analyzed: []string{}, OutsideScope: []string{}, Message: localLimitations}
	partial := !s.Complete
	for _, f := range s.Files {
		ext := path.Ext(f.Path)
		b := s.Data[f.Path]
		if ext != ".sh" && ext != ".bash" {
			e.OutsideScope = append(e.OutsideScope, f.Path)
			switch ext {
			case ".zsh", ".ksh", ".fish":
				partial = true
				s.Diagnostics = append(s.Diagnostics, Diagnostic{Scope: s.Scope, Path: f.Path, Code: "unsupported_shell", Message: "local rule supports .sh/.bash only", Incomplete: true})
			}
			continue
		}
		if !supportedShebang(b) {
			partial = true
			s.Diagnostics = append(s.Diagnostics, Diagnostic{Scope: s.Scope, Path: f.Path, Code: "unsupported_shebang", Message: "shell suffix has an unsupported interpreter or shebang arguments", Incomplete: true})
			continue
		}
		tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(b), f.Path)
		if err != nil {
			partial = true
			s.Diagnostics = append(s.Diagnostics, Diagnostic{Scope: s.Scope, Path: f.Path, Code: "shell_parse", Message: err.Error(), Incomplete: true})
			continue
		}
		e.Analyzed = append(e.Analyzed, f.Path)
		syntax.Walk(tree, func(node syntax.Node) bool {
			pipe, ok := node.(*syntax.BinaryCmd)
			if !ok || pipe.Op != syntax.Pipe {
				return true
			}
			from, to := pipelineEdge(pipe.X, true), pipelineEdge(pipe.Y, false)
			left, lok := command(from, 1)
			right, rok := command(to, 0)
			if !lok || !rok || !download(left) || !shell(right) {
				return true
			}
			start, end := int(from.Pos().Offset()), int(to.End().Offset())
			if end > len(b) {
				end = len(b)
			}
			evidence := string(b[start:end])
			if len(evidence) > 240 {
				end := 240
				for end > 0 && !utf8.RuneStart(evidence[end]) {
					end--
				}
				evidence = evidence[:end] + "…"
			}
			s.Findings = append(s.Findings, Finding{Scanner: "skill-audit", ScannerVersion: LocalVersion, RuleID: "SA001", RuleVersion: LocalRuleVersion, Severity: "high", Kind: "security", Path: f.Path, Line: int(from.Pos().Line()), Evidence: evidence, Explanation: "A literal HTTP(S) download is piped directly to a shell interpreter; remote bytes may execute with the caller's privileges.", Limitations: localLimitations})
			return true
		})
	}
	e.Status = finishStatus(len(s.Findings)-initialFindings, partial)
	s.Evaluations = append(s.Evaluations, e)
}

// Recognize the interpreter, not the end of the line. This is an analysis
// eligibility check, not an OS exec simulation. Environment assignments, quoted
// env split strings and unknown flags remain unsupported. No script is executed.
func supportedShebang(b []byte) bool {
	first, _, _ := bytes.Cut(b, []byte("\n"))
	if !bytes.HasPrefix(first, []byte("#!")) {
		return true
	}
	fields := strings.Fields(string(first[2:]))
	if len(fields) == 0 {
		return false
	}
	if path.Base(fields[0]) == "env" {
		fields = fields[1:]
		if len(fields) > 0 && fields[0] == "-S" {
			fields = fields[1:]
		}
	}
	if len(fields) == 0 || (path.Base(fields[0]) != "sh" && path.Base(fields[0]) != "bash") {
		return false
	}
	for _, arg := range fields[1:] {
		// Common flag-only options that still read the supplied script. Commands
		// (-c), option values (-o), alternate script operands and rc files are outside scope.
		if len(arg) < 2 || (arg[0] != '-' && arg[0] != '+') {
			return false
		}
		for _, flag := range arg[1:] {
			if !strings.ContainsRune("abefhkmnptuvxBCEHPT", flag) {
				return false
			}
		}
	}
	return true
}

// Each pipeline operator connects the last stage on its left to the first on its
// right. Inspecting that edge once avoids duplicate findings in longer chains.
func pipelineEdge(s *syntax.Stmt, last bool) *syntax.Stmt {
	for len(s.Redirs) == 0 {
		pipe, ok := s.Cmd.(*syntax.BinaryCmd)
		if !ok || (pipe.Op != syntax.Pipe && pipe.Op != syntax.PipeAll) {
			break
		}
		if last {
			s = pipe.Y
		} else {
			s = pipe.X
		}
	}
	return s
}

// Only literal simple commands whose relevant pipe descriptor is preserved
// qualify. The AST walk still visits nested commands, never literal text nodes.
func command(s *syntax.Stmt, connectedFD int) ([]string, bool) {
	if !preservesPipe(s.Redirs, connectedFD) {
		return nil, false
	}
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(c.Assigns) > 0 {
		return nil, false
	}
	args := []string{}
	for _, w := range c.Args {
		v, ok := literal(w.Parts)
		if !ok {
			return nil, false
		}
		args = append(args, v)
	}
	return args, len(args) > 0
}

func preservesPipe(redirs []*syntax.Redirect, connectedFD int) bool {
	for _, r := range redirs {
		fd := 1
		switch r.Op {
		case syntax.RdrAll, syntax.AppAll:
			if connectedFD == 1 {
				return false
			}
			continue
		case syntax.RdrIn, syntax.RdrInOut, syntax.DplIn, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
			fd = 0
		case syntax.RdrOut, syntax.AppOut, syntax.ClbOut, syntax.DplOut:
		default:
			return false
		}
		if r.N != nil {
			var err error
			fd, err = strconv.Atoi(r.N.Value)
			if err != nil || fd < 0 {
				return false
			} // variable descriptors are outside scope
		}
		if fd == connectedFD {
			return false
		}
		if r.Op == syntax.DplIn || r.Op == syntax.DplOut {
			v, ok := literal(r.Word.Parts)
			if !ok {
				return false
			}
			if v == "-" {
				continue
			} // closes only the unrelated destination
			if v == "" || strings.Trim(v, "0123456789") != "" {
				return false
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return false
			} // reject moves such as 2>&1-
		}
	}
	return true
}
func literal(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, p := range parts {
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
			v, ok := literal(x.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(v)
		default:
			return "", false
		}
	}
	return b.String(), true
}
func httpURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
func download(a []string) bool {
	if len(a) < 2 {
		return false
	}
	name := a[0]
	if name != "curl" && name != "wget" {
		return false
	}
	urls := 0
	stdout := name == "curl"
	endOpts := false
	for i := 1; i < len(a); i++ {
		v := a[i]
		if httpURL(v) {
			urls++
			continue
		}
		if v == "--" {
			endOpts = true
			continue
		}
		if endOpts {
			return false
		}
		if name == "curl" {
			switch v {
			case "--fail", "--silent", "--show-error", "--location":
				continue
			default:
				if len(v) > 1 && v[0] == '-' && !strings.HasPrefix(v, "--") && strings.Trim(v[1:], "fsSL") == "" {
					continue
				}
				return false
			}
		}
		switch v {
		case "-q", "--quiet", "--no-verbose":
			continue
		case "-O", "--output-document":
			if i+1 >= len(a) || a[i+1] != "-" {
				return false
			}
			stdout = true
			i++
		case "-O-", "-qO-", "--output-document=-":
			stdout = true
		default:
			return false
		}
	}
	return urls == 1 && stdout
}
func shell(a []string) bool {
	if len(a) == 0 || (a[0] != "sh" && a[0] != "bash") {
		return false
	}
	stdin, endOpts := false, false
	for _, v := range a[1:] {
		if endOpts {
			return stdin
		} // first post-options argument is otherwise a filename
		switch v {
		case "-s":
			stdin = true
		case "--":
			endOpts = true
		default:
			if !strings.HasPrefix(v, "-") {
				return stdin
			}
			return false
		}
	}
	return true
}
