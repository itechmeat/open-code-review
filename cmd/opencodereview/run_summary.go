// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"fmt"
	"time"

	"github.com/alibaba/open-code-review/internal/session"
)

// machineRunSummary is the one stderr line a json/sarif run prints, so a
// caller reading only stderr can tell a thorough zero-finding review from a
// skipped or shallow one without parsing the report.
func machineRunSummary(m *session.RunManifest, comments int, toolCalls map[string]int64,
	id *jsonLLMIdentity, elapsed time.Duration, sessionID string) string {
	status, files := "unknown", "0/0"
	if m != nil {
		status = string(m.TerminalState)
		done := len(m.Coverage.Completed) + len(m.Coverage.Reused)
		files = fmt.Sprintf("%d/%d", done, len(m.Coverage.Selected))
	}
	var calls int64
	for _, n := range toolCalls {
		calls += n
	}
	provider, model := "-", "-"
	if id != nil {
		if id.Provider != "" {
			provider = id.Provider
		}
		if id.Model != "" {
			model = id.Model
		}
	}
	if sessionID == "" {
		sessionID = "-"
	}
	return fmt.Sprintf("[ocr] Summary: status=%s files=%s comments=%d tool_calls=%d provider=%s model=%s elapsed=%s session=%s",
		status, files, comments, calls, provider, model, elapsed.Round(time.Second), sessionID)
}
