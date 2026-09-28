// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
	"github.com/alibaba/open-code-review/internal/session"
	"github.com/alibaba/open-code-review/internal/tool"
)

// limitAfterClient answers the first okCalls requests with task_done and every
// later one with a subscription usage limit.
type limitAfterClient struct {
	okCalls int64
	calls   int64 // atomic
}

func (c *limitAfterClient) CompletionsWithCtx(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if n := atomic.AddInt64(&c.calls, 1); n <= c.okCalls {
		return (&fakeBudgetClient{}).CompletionsWithCtx(ctx, req)
	}
	return nil, fmt.Errorf("LLM completion error: %w", llm.ErrClaudeCodeUsageLimit)
}

func newRunStopScanAgent(t *testing.T, client llm.LLMClient, tpl template.ScanTemplate, skipPlan bool) *Agent {
	t.Helper()
	a := NewAgent(Args{
		Template:         tpl,
		LLMClient:        client,
		CommentCollector: tool.NewCommentCollector(),
		Tools:            tool.NewRegistry(),
		MaxConcurrency:   1,
		Provider:         "claude-code",
		Session:          session.New(t.TempDir(), "main", "test", session.SessionOptions{ReviewMode: session.ReviewModeFullScan}),
		SkipPlan:         skipPlan,
		SkipDedup:        true,
		SkipSummary:      true,
	})
	a.items = []model.ScanItem{
		{Path: "a/one.go", Content: "package a\n", LineCount: 1},
		{Path: "b/two.go", Content: "package b\n", LineCount: 1},
		{Path: "b/three.go", Content: "package b\n", LineCount: 1},
		{Path: "c/four.go", Content: "package c\n", LineCount: 1},
	}
	a.args.Tools.Freeze()
	return a
}

func TestScanDispatchStopsOnFatalProviderError(t *testing.T) {
	client := &limitAfterClient{okCalls: 1}
	tpl := budgetTestTemplate()
	tpl.BatchStrategy = string(BatchByDirectory)
	a := newRunStopScanAgent(t, client, tpl, true)

	comments, err := a.dispatchSubtasks(context.Background())
	if err != nil {
		t.Fatalf("a scan with a scanned file must not fail: %v", err)
	}
	if comments == nil {
		t.Error("the results of the scanned file must be kept")
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 2 {
		t.Fatalf("LLM calls = %d, want 2: the usage limit must stop dispatch within and across batches", calls)
	}
	if !errors.Is(a.StoppedBy(), llm.ErrClaudeCodeUsageLimit) {
		t.Fatalf("StoppedBy() = %v", a.StoppedBy())
	}
	msg := a.RunStopMessage()
	if !strings.HasPrefix(msg, "claude-code cannot serve further requests") || !strings.Contains(msg, "usage limit") ||
		strings.Contains(msg, "LLM completion error") {
		t.Fatalf("RunStopMessage() = %q", msg)
	}
	stopped := 0
	for _, w := range a.Warnings() {
		if w.Type == "run_stopped" {
			stopped++
		}
	}
	if stopped != 1 {
		t.Errorf("run_stopped warnings = %d, want 1", stopped)
	}
}

func TestScanStopsWhenThePlanCallHitsAFatalError(t *testing.T) {
	client := &limitAfterClient{}
	tpl := budgetTestTemplate()
	tpl.PlanTask = &template.LlmConversation{Messages: []template.ChatMessage{{Role: "user", Content: "plan {{file_content}}"}}}
	a := newRunStopScanAgent(t, client, tpl, false)

	if _, err := a.dispatchSubtasks(context.Background()); !errors.Is(err, llm.ErrFatalForRun) {
		t.Fatalf("err = %v, want the fatal provider error", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 1 {
		t.Fatalf("LLM calls = %d, want 1: no main task after a fatal plan error", calls)
	}
}

// throttledClient fails every request with a transient rate limit, which a
// client gives up on per request but which must not stop the run.
type throttledClient struct{ calls int64 }

func (c *throttledClient) CompletionsWithCtx(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	atomic.AddInt64(&c.calls, 1)
	return nil, fmt.Errorf("LLM completion error: %w", llm.ErrClaudeCodeRateLimited)
}

func TestScanKeepsDispatchingAfterATransientError(t *testing.T) {
	client := &throttledClient{}
	a := newRunStopScanAgent(t, client, budgetTestTemplate(), true)

	_, err := a.dispatchSubtasks(context.Background())
	if err == nil || errors.Is(err, llm.ErrFatalForRun) {
		t.Fatalf("err = %v, want the all-failed error without a run stop", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 4 {
		t.Fatalf("LLM calls = %d, want every file dispatched", calls)
	}
	if a.StoppedBy() != nil || a.RunStopMessage() != "" {
		t.Fatalf("StoppedBy() = %v, RunStopMessage() = %q", a.StoppedBy(), a.RunStopMessage())
	}
}
