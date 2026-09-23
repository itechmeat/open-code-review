// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package session

import "time"

// AddTimedToolResult records a successful tool call together with how long it
// ran; AddToolResult stays for calls with no meaningful duration, such as a
// comment handed off to an asynchronous worker.
func (tr *TaskRecord) AddTimedToolResult(toolName, arguments, result string, duration time.Duration) {
	tr.addToolResult(toolName, arguments, result, true, duration)
}
