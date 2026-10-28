//go:build !darwin && !linux

package audit

import "os/exec"

// CommandContext still cancels the direct child; process-tree cancellation is
// supported only on macOS/Linux in this MVP.
func configureProcess(cmd *exec.Cmd) {}
