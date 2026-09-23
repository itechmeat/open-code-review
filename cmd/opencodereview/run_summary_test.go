// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/alibaba/open-code-review/internal/session"
)

func TestMachineRunSummary(t *testing.T) {
	m := &session.RunManifest{TerminalState: session.TerminalState("complete")}
	m.Coverage.Selected = make([]session.CoverageItem, 4)
	m.Coverage.Completed = make([]session.CoverageItem, 4)
	got := machineRunSummary(m, 2, map[string]int64{"code_search": 3, "task_done": 1},
		&jsonLLMIdentity{Provider: "claude-code", Model: "opus"}, 54*time.Second, "caae40c0")
	for _, want := range []string{"status=complete", "files=4/4", "comments=2", "tool_calls=4",
		"provider=claude-code", "model=opus", "elapsed=54s", "session=caae40c0"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
	if !strings.HasPrefix(got, "[ocr] Summary: ") || strings.Contains(got, "\n") {
		t.Errorf("summary must be one [ocr] line, got %q", got)
	}

	// No manifest (e.g. nothing selected) still yields a usable line.
	got = machineRunSummary(nil, 0, nil, nil, time.Second, "")
	if !strings.Contains(got, "status=unknown") || !strings.Contains(got, "tool_calls=0") {
		t.Errorf("summary without manifest = %q", got)
	}
}
