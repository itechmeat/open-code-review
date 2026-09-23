// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/agent"
)

func TestPreviewHintsHowToIncludeExcludedFiles(t *testing.T) {
	p := &agent.DiffPreview{TotalFiles: 2, ReviewableCount: 1, ExcludedCount: 1, Entries: []agent.DiffPreviewEntry{
		{Path: "a.ts", Status: "M", WillReview: true},
		{Path: "PILOT-NOTES.md", Status: "M", ExcludeReason: agent.ExcludeExtension},
	}}
	var sb strings.Builder
	outputPreviewText(p, &sb)
	if !strings.Contains(sb.String(), `"include"`) || !strings.Contains(sb.String(), "ocr rules check") {
		t.Errorf("preview must say how to bring excluded files back:\n%s", sb.String())
	}

	p.Entries[1].ExcludeReason = agent.ExcludeSecret
	sb.Reset()
	outputPreviewText(p, &sb)
	if strings.Contains(sb.String(), `"include"`) {
		t.Errorf("no include hint for exclusions include cannot undo:\n%s", sb.String())
	}
}
