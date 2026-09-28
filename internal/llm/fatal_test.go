// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestClaudeCodeUsageLimitIsFatalForRun(t *testing.T) {
	limit := classifyClaudeCodeFailure("Claude AI usage limit reached|1760000000")
	wrapped := fmt.Errorf("LLM completion error: %w", limit)
	if !errors.Is(wrapped, ErrFatalForRun) || !errors.Is(wrapped, ErrClaudeCodeUsageLimit) {
		t.Fatalf("limit error %v must match both ErrFatalForRun and ErrClaudeCodeUsageLimit", wrapped)
	}
	for _, msg := range []string{"Not logged in · Please run /login", "API Error: overloaded"} {
		if err := classifyClaudeCodeFailure(msg); errors.Is(err, ErrFatalForRun) {
			t.Errorf("%q must not be fatal for the run", msg)
		}
	}
	if ErrClaudeCodeUsageLimit.Error() == ErrFatalForRun.Error() {
		t.Fatal("the specific message must be kept")
	}
}
