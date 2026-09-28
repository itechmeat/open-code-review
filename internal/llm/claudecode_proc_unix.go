// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

//go:build !windows

package llm

import (
	"os/exec"
	"syscall"
)

// setClaudeCodeProcAttr puts the CLI in its own process group so a cancelled
// review also takes down anything the CLI spawned. The CLI needs no terminal,
// so the TTY concerns that keep keycmd off Setpgid do not apply here.
func setClaudeCodeProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
