// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package session

import (
	"testing"
	"time"
)

func TestAddTimedToolResultPersistsDuration(t *testing.T) {
	setTestHome(t, t.TempDir())
	repoDir := t.TempDir()
	sh := New(repoDir, "main", "model", SessionOptions{})
	rec := sh.GetOrCreateFileSession("file.go").AppendTaskRecord(MainTask, nil)

	rec.AddTimedToolResult("file_read", `{"path":"a.go"}`, "package a", 40*time.Millisecond)

	if got := rec.ToolResults[0]; !got.OK || got.Duration != 40*time.Millisecond {
		t.Fatalf("result = %+v", got)
	}
	if err := sh.Finalize(); err != nil {
		t.Fatal(err)
	}
	for _, record := range readJSONLRecords(t, sessionJSONLPath(t, repoDir, sh.SessionID)) {
		if record["type"] == "tool_call" {
			if ms, _ := record["duration_ms"].(float64); ms != 40 {
				t.Fatalf("duration_ms = %v, want 40", record["duration_ms"])
			}
			return
		}
	}
	t.Fatal("tool call was not persisted")
}
