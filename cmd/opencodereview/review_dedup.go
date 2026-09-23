// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"context"
	"fmt"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
	"github.com/alibaba/open-code-review/internal/scan"
	"github.com/alibaba/open-code-review/internal/stdout"
)

// dedupReviewComments merges findings that different file groups reported
// about the same problem, reusing scan's DEDUP_TASK. The session keeps the
// raw per-file comments; only the reported set is merged. Any failure keeps
// the comments as they are. The usage of the extra call is returned so it
// counts toward the run's token totals.
func dedupReviewComments(ctx context.Context, client llm.LLMClient, modelName string, maxTokens int,
	comments []model.LlmComment, skip bool) ([]model.LlmComment, *llm.UsageInfo) {
	if skip {
		return comments, nil
	}
	tpl, err := template.LoadScanDefault()
	if err != nil || tpl.DedupTask == nil || len(tpl.DedupTask.Messages) == 0 {
		return comments, nil
	}
	minN := tpl.DedupMinComments
	if minN <= 0 {
		minN = 2
	}
	if len(comments) < minN {
		return comments, nil
	}
	out, usage, err := scan.DedupComments(ctx, client, modelName, tpl.DedupTask, comments, maxTokens)
	if err != nil {
		fmt.Fprintf(stdout.Writer(), "[ocr] Dedup skipped: %v\n", err)
		return comments, usage
	}
	if len(out) < len(comments) {
		fmt.Fprintf(stdout.Writer(), "[ocr] Dedup: %d → %d comments\n", len(comments), len(out))
	}
	return out, usage
}
