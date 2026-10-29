package audit

import (
	"os"
	"path/filepath"
	"runtime"
)

func exclusionRule(p string, i os.FileInfo, explicit bool) string {
	n := filepath.Base(p)
	if Metadata(n) {
		return "metadata/" + n
	}
	if (i.IsDir() || isLink(i)) && Dependency(n) && !explicit {
		return "dependency/" + n
	}
	for _, x := range []string{"/proc", "/sys", "/dev"} {
		if runtime.GOOS != "windows" && Within(x, p) {
			return "system" + x
		}
	}
	if runtime.GOOS == "darwin" {
		if Within("/System/Volumes/Data", p) {
			return "system/System/Volumes/Data"
		}
		if !explicit {
			for _, x := range []string{"/etc", "/tmp", "/var"} {
				if p == x {
					return "system" + x
				}
			}
		}
	}
	return ""
}

// directoryContains confirms ancestor identity when lexical spelling differs
// (for example case aliases on a case-insensitive volume). It never merges
// local package identities, which remain scope plus lexical path.
func directoryContains(root, path string) bool {
	root = Abs(root)
	path = Abs(path)
	if Within(root, path) {
		return true
	}
	if NoLinks(root) != nil || NoLinks(path) != nil {
		return false
	}
	base, e := os.Lstat(root)
	if e != nil || !base.IsDir() {
		return false
	}
	for q := filepath.Dir(path); ; q = filepath.Dir(q) {
		i, err := os.Lstat(q)
		if err == nil && os.SameFile(base, i) {
			return true
		}
		if filepath.Dir(q) == q {
			break
		}
	}
	return false
}
