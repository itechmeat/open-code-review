// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

//go:build windows

package llm

import "os/exec"

// setClaudeCodeProcAttr keeps exec's default cancel (Process.Kill) on Windows,
// which has no process groups to signal.
func setClaudeCodeProcAttr(_ *exec.Cmd) {}
