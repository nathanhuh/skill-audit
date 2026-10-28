//go:build !windows

package audit

import "os"

func syncStateDirectory(parent string) error {
	d, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
