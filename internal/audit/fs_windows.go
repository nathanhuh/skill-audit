//go:build windows

package audit

import (
	"os"
	"syscall"
)

func isLink(i os.FileInfo) bool {
	if i.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if a, ok := i.Sys().(*syscall.Win32FileAttributeData); ok {
		return a.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}
func syncDirectory(string) error { return nil } // NTFS replacement and crash durability remain unverified.
func DeviceRoots() []string {
	var r []string
	for c := 'A'; c <= 'Z'; c++ {
		p := string(c) + ":\\"
		if _, e := os.Lstat(p); e == nil {
			r = append(r, p)
		}
	}
	return r
}
