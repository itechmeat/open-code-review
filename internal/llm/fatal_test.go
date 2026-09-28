// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestClaudeCodeAccountErrorsAreFatalForRun(t *testing.T) {
	for msg, sentinel := range map[string]error{
		"Claude AI usage limit reached|1760000000": ErrClaudeCodeUsageLimit,
		"Not logged in · Please run /login":        ErrClaudeCodeNotLoggedIn,
		"error: unknown option '--effort'":         ErrClaudeCodeOutdated,
	} {
		wrapped := fmt.Errorf("LLM completion error: %w", classifyClaudeCodeFailure(msg))
		if !errors.Is(wrapped, ErrFatalForRun) || !errors.Is(wrapped, sentinel) {
			t.Errorf("%q: %v must match both ErrFatalForRun and %v", msg, wrapped, sentinel)
		}
	}
	if err := classifyClaudeCodeFailure("API Error: overloaded"); errors.Is(err, ErrFatalForRun) {
		t.Error("an unclassified API error must not be fatal for the run")
	}
}

func TestFatalCauseDropsOuterWrapping(t *testing.T) {
	cause := classifyClaudeCodeFailure("Not logged in · Please run /login")
	got := FatalCause(fmt.Errorf("group g1: LLM completion error: %w", cause))
	if got.Error() != ErrClaudeCodeNotLoggedIn.Error()+": Not logged in · Please run /login" {
		t.Fatalf("FatalCause() = %q", got)
	}
	if FatalCause(ErrClaudeCodeUsageLimit) != ErrClaudeCodeUsageLimit {
		t.Error("a bare sentinel is its own cause")
	}
	plain := errors.New("boom")
	if FatalCause(plain) != plain {
		t.Error("a non-fatal error is returned unchanged")
	}
}

func TestRunStopKeepsTheFirstFatalError(t *testing.T) {
	var s RunStop
	if fatal, first := s.Record(errors.New("boom")); fatal || first || s.Err() != nil {
		t.Fatal("a non-fatal error must not stop the run")
	}
	limit := fmt.Errorf("group g1: %w", classifyClaudeCodeFailure("You've hit your session limit · resets 5pm (Europe/Belgrade)"))
	if fatal, first := s.Record(limit); !fatal || !first {
		t.Fatalf("Record(limit) = %v, %v, want the first fatal error", fatal, first)
	}
	if fatal, first := s.Record(ErrClaudeCodeNotLoggedIn); !fatal || first {
		t.Fatalf("Record(login) = %v, %v, want a fatal but not first error", fatal, first)
	}
	if s.Err() != limit {
		t.Fatalf("Err() = %v, want the first fatal error", s.Err())
	}
	want := "claude-code cannot serve further requests, so the remaining files were not dispatched: " +
		"Claude usage limit reached (wait for the limit to reset or lower --concurrency): resets 5pm (Europe/Belgrade)"
	if got := RunStopMessage("claude-code", s.Err()); got != want {
		t.Errorf("RunStopMessage() = %q, want %q", got, want)
	}
	if got := RunStopMessage("", s.Err()); !strings.HasPrefix(got, "the LLM provider cannot serve") {
		t.Errorf("RunStopMessage() without a provider = %q", got)
	}
	if RunStopMessage("claude-code", nil) != "" {
		t.Error("no stop, no message")
	}
}
