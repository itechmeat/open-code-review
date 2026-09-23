// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/template"
)

func TestAppendReviewGuidance(t *testing.T) {
	tpl, err := template.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	before := tpl.MainTask.Messages[0].Content
	appendReviewGuidance(tpl)
	appendReviewGuidance(tpl) // idempotent
	got := tpl.MainTask.Messages[0].Content
	if tpl.MainTask.Messages[0].Role != "system" || !strings.HasPrefix(got, before) {
		t.Fatal("guidance must extend the main task's system prompt")
	}
	for _, want := range []string{"which side is wrong", "## Contradictions and generated code"} {
		if strings.Count(got, want) != 1 {
			t.Errorf("guidance %q appears %d times", want, strings.Count(got, want))
		}
	}
}

func TestReviewDedupPromptMergesSharedRootCauses(t *testing.T) {
	conv := reviewDedupConversation()
	if conv == nil || !strings.Contains(conv.Messages[0].Content, "root cause") {
		t.Fatal("review dedup must merge findings that share one root cause")
	}
	scanTpl, err := template.LoadScanDefault()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(scanTpl.DedupTask.Messages[0].Content, "root cause") {
		t.Error("scan's own dedup prompt must stay untouched")
	}
}

func TestReviewGuidanceRejectsToolLimitClaims(t *testing.T) {
	tpl, err := template.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	appendReviewGuidance(tpl)
	if !strings.Contains(tpl.MainTask.Messages[0].Content, "could not be checked") {
		t.Error("the main prompt must forbid reporting tool limits as findings")
	}
	var filterSystem string
	for _, m := range tpl.ReviewFilterTask.Messages {
		if m.Role == "system" {
			filterSystem = m.Content
		}
	}
	if strings.Count(filterSystem, "reviewer's tools") != 1 {
		t.Errorf("the filter must remove tool-limit comments exactly once:\n%s", filterSystem)
	}
	appendReviewGuidance(tpl)
	for _, m := range tpl.ReviewFilterTask.Messages {
		if m.Role == "system" && strings.Count(m.Content, "reviewer's tools") != 1 {
			t.Error("filter guidance must be idempotent")
		}
	}
}
