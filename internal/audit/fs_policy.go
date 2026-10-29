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
