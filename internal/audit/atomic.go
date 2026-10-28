package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic flushes a same-directory temporary file before replacement and
// syncs the directory where supported. Callers own size, placement and locking
// policy. Cancellation before rename leaves the previous destination intact.
func WriteAtomic(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := NoSymlinks(path); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	tmp, err := os.CreateTemp(parent, ".skill-audit-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if err = syncDirectory(parent); err != nil {
		return fmt.Errorf("file replaced, but directory durability could not be confirmed; inspect destination before retrying: %w", err)
	}
	return nil
}
