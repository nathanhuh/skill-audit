package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const MaxFile = 1 << 20
const MaxPackage = 16 << 20
const MaxEntries = 1000

// NoSymlinks checks existing path components, including the selected root.
// This is a best-effort filesystem walk, not protection from concurrent hostile mutation.
func NoSymlinks(path string) error {
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		i, e := os.Lstat(p)
		if e == nil && i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component: %s", p)
		}
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return nil
}
func ReadBounded(path string, limit int64) ([]byte, error) {
	if err := NoSymlinks(path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = fmt.Errorf("file exceeds %d bytes", limit)
	}
	return b, err
}
func excluded(name string) bool {
	return name == ".git" || name == ".hg" || name == ".svn" || name == ".DS_Store"
}
func Inspect(ctx context.Context, scope, path string) Skill {
	s := Skill{Scope: scope, Path: path, Name: filepath.Base(path), Complete: true, Files: []File{}, Data: map[string][]byte{}, Diagnostics: []Diagnostic{}, Findings: []Finding{}, Evaluations: []Evaluation{}}
	issue := func(p, code, msg string, incomplete bool) {
		s.Diagnostics = append(s.Diagnostics, Diagnostic{scope, p, code, msg, incomplete})
		if incomplete {
			s.Complete = false
		}
	}
	if err := ctx.Err(); err != nil {
		issue(path, "inspection_canceled", err.Error(), true)
		return s
	}
	if err := NoSymlinks(path); err != nil {
		issue(path, "symlink", err.Error(), true)
		return s
	}
	count := 0
	total := 0
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		rel, _ := filepath.Rel(path, p)
		rel = filepath.ToSlash(rel)
		if err != nil {
			issue(rel, "read_error", err.Error(), true)
			return nil
		}
		if rel == "." {
			return nil
		}
		if !utf8.ValidString(rel) {
			issue(rel, "invalid_path_encoding", "path is not UTF-8; skipped", true)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		count++
		if count > MaxEntries {
			issue(rel, "entry_limit", "package exceeds 1000 entries; remaining traversal stopped", true)
			return fs.SkipAll
		}
		if d.Type()&os.ModeSymlink != 0 {
			issue(rel, "symlink", "symlink skipped", true)
			return nil
		}
		if excluded(d.Name()) {
			issue(rel, "excluded_metadata", "excluded from fingerprint and scanning", false)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			issue(rel, "special_file", "special file skipped", true)
			return nil
		}
		b, err := ReadBounded(p, MaxFile)
		if err != nil {
			issue(rel, "read_error", err.Error(), true)
			return nil
		}
		if total+len(b) > MaxPackage {
			issue(rel, "byte_limit", "package exceeds 16 MiB; remaining traversal stopped", true)
			return fs.SkipAll
		}
		total += len(b)
		s.Data[rel] = b
		s.Files = append(s.Files, File{rel, hash(b), int64(len(b))})
		return nil
	})
	if ctx.Err() != nil {
		issue(path, "inspection_canceled", ctx.Err().Error(), true)
	} else if err != nil {
		issue(path, "walk_error", err.Error(), true)
	}
	if b, ok := s.Data["SKILL.md"]; ok {
		if name, err := metadata(b); err != nil {
			issue("SKILL.md", "metadata", err.Error(), true)
		} else if name != "" {
			s.Name = name
		}
	} else {
		issue("SKILL.md", "manifest_unread", "SKILL.md was not read", true)
	}
	if s.Complete {
		s.Digest = Digest(s.Files)
	}
	return s
}
func metadata(b []byte) (string, error) {
	lines := bytes.Split(b, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return "", nil
	}
	end := 0
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			end = i
			break
		}
	}
	if end == 0 {
		return "", fmt.Errorf("unterminated YAML frontmatter")
	}
	var m map[string]any
	if err := yaml.Unmarshal(bytes.Join(lines[1:end], []byte("\n")), &m); err != nil {
		return "", fmt.Errorf("invalid frontmatter: %w", err)
	}
	name := ""
	for _, k := range []string{"name", "description"} {
		if v, ok := m[k]; ok {
			str, ok := v.(string)
			if !ok || strings.TrimSpace(str) == "" {
				return "", fmt.Errorf("%s must be a nonempty string", k)
			}
			if k == "name" {
				name = str
			}
		}
	}
	return name, nil
}
