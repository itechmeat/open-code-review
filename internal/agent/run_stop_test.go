// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/session"
	"github.com/alibaba/open-code-review/internal/tool"
)

// limitAfterClient answers the first okCalls requests with ok and every later
// one with fail, a subscription usage limit by default.
type limitAfterClient struct {
	okCalls int64
	ok      func(path string) *llm.ChatResponse
	path    string
	fail    error
	calls   int64 // atomic
}

func (c *limitAfterClient) CompletionsWithCtx(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if n := atomic.AddInt64(&c.calls, 1); n <= c.okCalls {
		return c.ok(c.path), nil
	}
	cause := c.fail
	if cause == nil {
		cause = llm.ErrClaudeCodeUsageLimit
	}
	return nil, fmt.Errorf("LLM completion error: %w", cause)
}

func commentAndDone(path string) *llm.ChatResponse {
	return (&fakeCommentAndDoneClient{path: path}).response()
}

func (f *fakeCommentAndDoneClient) response() *llm.ChatResponse {
	resp, _ := f.CompletionsWithCtx(context.Background(), llm.ChatRequest{})
	return resp
}

// perFileTemplate dispatches every file as its own group without a grouping
// call, so the number of LLM calls equals the number of dispatched rounds.
func perFileTemplate() template.Template {
	tpl := budgetAgentTestTemplate()
	tpl.GroupingMinFiles = 100
	tpl.GroupingBundleLineThreshold = 0
	return tpl
}

func newRunStopAgent(t *testing.T, client llm.LLMClient, tpl template.Template, n int, provider string) *Agent {
	t.Helper()
	setTestHome(t, t.TempDir())
	collector := tool.NewCommentCollector()
	reg := tool.NewRegistry()
	reg.Register(&tool.CodeCommentProvider{Collector: collector})
	a := New(Args{
		LLMClient:        client,
		Model:            "fake",
		Provider:         provider,
		CommentCollector: collector,
		Tools:            reg,
		MaxConcurrency:   1,
		SkipFilter:       true,
		Template:         tpl,
		MainToolDefs: []llm.ToolDef{
			{Type: "function", Function: llm.FunctionDef{Name: "task_done", Description: "done"}},
			{Type: "function", Function: llm.FunctionDef{Name: "code_comment", Description: "comment"}},
		},
	})
	a.diffs = makeBudgetDiffs(n)
	a.currentDate = "2026-09-28 10:00"
	a.args.Tools.Freeze()
	return a
}

func finishRunStop(t *testing.T, a *Agent) *session.RunManifest {
	t.Helper()
	if err := a.finalizeManifest(); err != nil {
		t.Fatalf("finalize manifest: %v", err)
	}
	if err := a.session.Finalize(); err != nil {
		t.Fatalf("finalize session: %v", err)
	}
	return a.RunManifest()
}

func TestDispatchStopsOnFatalErrorAndKeepsReviewedFiles(t *testing.T) {
	client := &limitAfterClient{okCalls: 1, ok: func(string) *llm.ChatResponse { return agentTaskDoneResponse() }}
	a := newRunStopAgent(t, client, perFileTemplate(), 3, "claude-code")

	if _, err := a.dispatchSubtasks(context.Background()); err != nil {
		t.Fatalf("a run with a reviewed file must not fail: %v", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 2 {
		t.Fatalf("LLM calls = %d, want 2: the third file must not be dispatched", calls)
	}
	if msg := a.RunStopMessage(); !strings.Contains(msg, "claude-code") || !strings.Contains(msg, "usage limit") {
		t.Fatalf("RunStopMessage() = %q", msg)
	}
	var stopped bool
	for _, w := range a.Warnings() {
		stopped = stopped || w.Type == "run_stopped"
	}
	if !stopped {
		t.Error("expected a run_stopped warning")
	}
	m := finishRunStop(t, a)
	if m.TerminalState != session.StatePartial || len(m.Coverage.Completed) != 1 || len(m.Coverage.Failed) != 2 {
		t.Fatalf("manifest state=%s completed=%d failed=%d", m.TerminalState, len(m.Coverage.Completed), len(m.Coverage.Failed))
	}
	for _, item := range m.Coverage.Failed {
		if item.Classification != session.FailureProvider {
			t.Errorf("item %s class = %q, want provider", item.ItemID, item.Classification)
		}
	}
}

func TestDispatchStopsWhenClaudeCodeIsNotLoggedIn(t *testing.T) {
	client := &limitAfterClient{okCalls: 1, ok: func(string) *llm.ChatResponse { return agentTaskDoneResponse() }, fail: llm.ErrClaudeCodeNotLoggedIn}
	a := newRunStopAgent(t, client, perFileTemplate(), 3, "claude-code")

	if _, err := a.dispatchSubtasks(context.Background()); err != nil {
		t.Fatalf("a run with a reviewed file must not fail: %v", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 2 {
		t.Fatalf("LLM calls = %d, want 2: a missing login must stop dispatch", calls)
	}
	msg := a.RunStopMessage()
	if !strings.HasPrefix(msg, "claude-code cannot serve further requests") || !strings.Contains(msg, "/login") ||
		strings.Contains(msg, "LLM completion error") || strings.Count(msg, "claude-code") != 1 {
		t.Fatalf("RunStopMessage() = %q", msg)
	}
	if m := finishRunStop(t, a); m.TerminalState != session.StatePartial || len(m.Coverage.Failed) != 2 {
		t.Fatalf("manifest state=%s failed=%d", m.TerminalState, len(m.Coverage.Failed))
	}
}

func TestDispatchStopsOnFatalErrorBeforeAnyReview(t *testing.T) {
	client := &limitAfterClient{}
	a := newRunStopAgent(t, client, perFileTemplate(), 3, "")

	_, err := a.dispatchSubtasks(context.Background())
	if !errors.Is(err, llm.ErrFatalForRun) || !strings.Contains(err.Error(), "before any file was reviewed") {
		t.Fatalf("err = %v, want the fatal provider error", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 1 {
		t.Fatalf("LLM calls = %d, want 1", calls)
	}
	if msg := a.RunStopMessage(); !strings.HasPrefix(msg, "the LLM provider") {
		t.Fatalf("RunStopMessage() = %q", msg)
	}
	if m := finishRunStop(t, a); len(m.Coverage.Failed) != 3 {
		t.Fatalf("failed = %d, want 3", len(m.Coverage.Failed))
	}
}

func TestFatalErrorInPlanPhaseSkipsTheMainTask(t *testing.T) {
	client := &limitAfterClient{}
	tpl := perFileTemplate()
	tpl.PlanTask = &template.LlmConversation{Messages: []template.ChatMessage{{Role: "user", Content: "plan {{diffs}}"}}}
	a := newRunStopAgent(t, client, tpl, 2, "claude-code")

	if _, err := a.dispatchSubtasks(context.Background()); !errors.Is(err, llm.ErrFatalForRun) {
		t.Fatalf("err = %v", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 1 {
		t.Fatalf("LLM calls = %d, want 1: neither the main task nor the next file may run", calls)
	}
	_ = finishRunStop(t, a)
}

func TestFatalErrorInLaterRoundKeepsFindingsAndStopsDispatch(t *testing.T) {
	client := &limitAfterClient{okCalls: 1, ok: commentAndDone, path: makeBudgetDiffs(1)[0].NewPath}
	tpl := perFileTemplate()
	tpl.MaxReviewRounds = 2
	a := newRunStopAgent(t, client, tpl, 2, "claude-code")

	comments, err := a.dispatchSubtasks(context.Background())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want the round 1 finding", len(comments))
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 2 {
		t.Fatalf("LLM calls = %d, want 2: round 2 fails and the second file is not dispatched", calls)
	}
	if a.StoppedBy() == nil {
		t.Fatal("a fatal error in a later round must stop the run")
	}
	m := finishRunStop(t, a)
	if len(m.Coverage.Completed) != 1 || len(m.Coverage.Failed) != 1 {
		t.Fatalf("completed=%d failed=%d, want 1 and 1", len(m.Coverage.Completed), len(m.Coverage.Failed))
	}
}

func TestStopRunOnIgnoresOrdinaryErrors(t *testing.T) {
	a := newRunStopAgent(t, &limitAfterClient{}, perFileTemplate(), 1, "")
	t.Cleanup(func() { _ = a.Session().Finalize() })
	if a.stopRunOn(nil) || a.stopRunOn(errors.New("boom")) {
		t.Fatal("only fatal errors stop the run")
	}
	if a.StoppedBy() != nil || a.RunStopMessage() != "" {
		t.Fatal("no stop was recorded")
	}
	first := fmt.Errorf("first: %w", llm.ErrClaudeCodeUsageLimit)
	if !a.stopRunOn(first) || !a.stopRunOn(fmt.Errorf("second: %w", llm.ErrClaudeCodeUsageLimit)) {
		t.Fatal("fatal errors must be recognized")
	}
	if a.StoppedBy() != first {
		t.Fatal("the first fatal error must be kept")
	}
}

// inFlightClient holds the first request until the run has been stopped by the
// second, which fails with a usage limit; every later request would fail too.
type inFlightClient struct {
	agent *Agent
	calls int64 // atomic
}

func (c *inFlightClient) CompletionsWithCtx(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if atomic.AddInt64(&c.calls, 1) > 1 {
		return nil, fmt.Errorf("LLM completion error: %w", llm.ErrClaudeCodeUsageLimit)
	}
	deadline := time.Now().Add(5 * time.Second)
	for c.agent.StoppedBy() == nil {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return nil, errors.New("the run was never stopped")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return agentTaskDoneResponse(), nil
}

func TestDispatchStopLetsInFlightGroupsFinish(t *testing.T) {
	client := &inFlightClient{}
	a := newRunStopAgent(t, client, perFileTemplate(), 4, "claude-code")
	client.agent = a
	a.args.MaxConcurrency = 2

	if _, err := a.dispatchSubtasks(context.Background()); err != nil {
		t.Fatalf("a run with a reviewed file must not fail: %v", err)
	}
	if calls := atomic.LoadInt64(&client.calls); calls != 2 {
		t.Fatalf("LLM calls = %d, want 2: nothing may be dispatched after the stop", calls)
	}
	m := finishRunStop(t, a)
	if len(m.Coverage.Completed) != 1 || len(m.Coverage.Failed) != 3 {
		t.Fatalf("completed=%d failed=%d, want the in-flight group completed and 3 failed", len(m.Coverage.Completed), len(m.Coverage.Failed))
	}
}
