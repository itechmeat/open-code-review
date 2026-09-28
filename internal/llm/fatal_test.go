// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"errors"
	"fmt"
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
