//go:build !windows

package audit

import "os"

func isLink(i os.FileInfo) bool { return i.Mode()&os.ModeSymlink != 0 }
func syncDirectory(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func DeviceRoots() []string { return []string{"/"} }
