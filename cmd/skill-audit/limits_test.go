package main

import (
	"bytes"
	"context"
	"testing"
)

func TestZeroLimitsRejectedBeforeWork(t *testing.T) {
	for _, args := range [][]string{
		{"inventory", "--root", "missing", "--max-entries", "0"},
		{"inventory", "--root", "missing", "--max-packages", "0"},
		{"inventory", "--root", "missing", "--max-depth", "0"},
		{"search", "--query", "example", "--limit", "0"},
		{"sync", "--repo", "owner/repo", "--limit", "0"},
		{"sync", "--repo", "owner/repo", "--public-requests", "0"},
	} {
		out, errout := new(bytes.Buffer), new(bytes.Buffer)
		if code := run(context.Background(), args, out, errout); code != 2 || out.Len() != 0 || errout.Len() == 0 {
			t.Fatal(args, code, out, errout)
		}
	}
}
