package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// CommittedWriteError means replacement succeeded, but directory durability
// could not be confirmed. Retrying the logical operation may duplicate it.
type CommittedWriteError struct {
	Path string
	Err  error
}

func (e *CommittedWriteError) Error() string {
	return fmt.Sprintf("file %q was replaced, but directory durability is unconfirmed: %v; inspect before retrying", e.Path, e.Err)
}
func (e *CommittedWriteError) Unwrap() error { return e.Err }

// WriteAtomic flushes a same-directory temporary file before replacement and
// syncs the directory where supported. Callers own size, placement and locking
// policy. Cancellation before rename leaves the previous destination intact.
func WriteAtomic(ctx context.Context, path string, data []byte) error {
	return writeAtomic(ctx, path, data, syncDirectory)
}

func writeAtomic(ctx context.Context, path string, data []byte, syncDir func(string) error) error {
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
	if err = syncDir(parent); err != nil {
		return &CommittedWriteError{Path: path, Err: err}
	}
	return nil
}
