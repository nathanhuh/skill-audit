//go:build windows

package audit

import (
	"os/exec"
	"time"
)

// Direct-child cancellation only. Job Objects and native descendant tests remain pending.
func configureProcess(cmd *exec.Cmd) { cmd.WaitDelay = time.Second }
