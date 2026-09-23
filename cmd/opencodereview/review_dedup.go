// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alibaba/open-code-review/internal/agent"
	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
	"github.com/alibaba/open-code-review/internal/scan"
	"github.com/alibaba/open-code-review/internal/stdout"
)

// dedupReviewComments merges findings that different file groups reported
// about the same problem, reusing scan's DEDUP_TASK. The session keeps the
// raw per-file comments; only the reported set is merged. Any failure keeps
// the comments as they are. It also returns the extra call's usage, so it
// counts toward the run totals, and a short status for the run summary.
func dedupReviewComments(ctx context.Context, client llm.LLMClient, modelName string, maxTokens int,
	comments []model.LlmComment, skip bool) ([]model.LlmComment, *llm.UsageInfo, string) {
	if skip {
		return comments, nil, "skipped (--no-dedup)"
	}
	tpl, err := template.LoadScanDefault()
	conv := reviewDedupConversation()
	if err != nil || conv == nil {
		return comments, nil, "unavailable"
	}
	minN := tpl.DedupMinComments
	if minN <= 0 {
		minN = 2
	}
	if len(comments) < minN {
		return comments, nil, fmt.Sprintf("skipped (%d findings)", len(comments))
	}
	start := time.Now()
	out, usage, err := scan.DedupComments(ctx, client, modelName, conv, comments, maxTokens)
	took := time.Since(start).Round(time.Second)
	if err != nil {
		fmt.Fprintf(stdout.Writer(), "[ocr] Dedup skipped: %v\n", err)
		return comments, usage, fmt.Sprintf("failed in %s, kept %d", took, len(comments))
	}
	if len(out) < len(comments) {
		fmt.Fprintf(stdout.Writer(), "[ocr] Dedup: %d → %d comments\n", len(comments), len(out))
		return out, usage, fmt.Sprintf("merged %d→%d in %s", len(comments), len(out), took)
	}
	return out, usage, fmt.Sprintf("no duplicates in %s", took)
}

// dedupAwareResult carries the dedup status to emitRunResult alongside the
// agent, whose other methods it promotes unchanged.
type dedupAwareResult struct {
	*agent.Agent
	status string
}

func (r dedupAwareResult) DedupStatus() string { return r.status }
