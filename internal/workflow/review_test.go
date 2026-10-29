package workflow

import "testing"

func TestMacOSAlternateDataTreePolicy(t *testing.T) {
	for _, tc := range []struct {
		path, root, goos string
		want             bool
	}{
		{"/System/Volumes/Data", "/", "darwin", true},
		{"/System/Volumes/Data/Users/alice", "/", "darwin", true},
		{"/System/Volumes/Data", "/System/Volumes/Data", "darwin", true},
		{"/System/Volumes/Data/Users", "/System/Volumes/Data/Users", "darwin", true},
		{"/Users/alice", "/", "darwin", false},
		{"/etc", "/", "darwin", true},
		{"/tmp", "/", "darwin", true},
		{"/var", "/", "darwin", true},
		{"/private/etc", "/", "darwin", false},
		{"/etc", "/", "linux", false},
		{"/System/Volumes/Database", "/", "darwin", false},
		{"/System/Volumes/Data", "/", "linux", false},
	} {
		if got := excludedOnOS(tc.path, "ordinary", tc.root, tc.goos); got != tc.want {
			t.Fatalf("%+v got %v", tc, got)
		}
	}
}

func TestMissingHomeDoesNotClassifyRelativeAgentDirectoryAsGlobal(t *testing.T) {
	if scope, _ := classify(".claude/skills/example", ""); scope == "global" {
		t.Fatal("empty home used working directory")
	}
}
