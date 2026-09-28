// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"errors"
	"testing"
)

func TestClassifyClaudeCodeFailureSeparatesTransientLimits(t *testing.T) {
	tests := []struct {
		msg      string
		sentinel error
		fatal    bool
	}{
		{"Claude AI usage limit reached|1760000000", ErrClaudeCodeUsageLimit, true},
		{"You've hit your limit · resets 3am", ErrClaudeCodeUsageLimit, true},
		{"API Error: 429 rate limit exceeded", ErrClaudeCodeRateLimited, false},
		{"API Error: Rate limit reached for requests", ErrClaudeCodeRateLimited, false},
		{`API Error: 429 {"type":"error","error":{"type":"rate_limit_error"}}`, ErrClaudeCodeRateLimited, false},
		{"API Error: Repeated 529 Overloaded errors", ErrClaudeCodeRateLimited, false},
		{`{"type":"overloaded_error","message":"Overloaded"}`, ErrClaudeCodeRateLimited, false},
		{"API Error: prompt is 14290 tokens too long", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := classifyClaudeCodeFailure(tt.msg)
			if got := errors.Is(err, ErrFatalForRun); got != tt.fatal {
				t.Errorf("fatal = %v, want %v (%v)", got, tt.fatal, err)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("err = %v, want %v", err, tt.sentinel)
			}
			if tt.sentinel == nil && (errors.Is(err, ErrClaudeCodeRateLimited) || errors.Is(err, ErrClaudeCodeUsageLimit)) {
				t.Errorf("err = %v, want an unclassified error", err)
			}
		})
	}
}

func TestClassifyClaudeCodeFailureStopsOnPlanLimitsWithResetTime(t *testing.T) {
	tests := []struct{ msg, detail string }{
		{"You've hit your session limit · resets 5pm (Europe/Belgrade)", "resets 5pm (Europe/Belgrade)"},
		{"You've hit your session limit - resets 5pm (Europe/Belgrade)", "resets 5pm (Europe/Belgrade)"},
		{"Session limit reached ∙ resets 11:30pm", "resets 11:30pm"},
		{"You've hit your limit · resets 3am (UTC)", "resets 3am (UTC)"},
		{"Claude AI usage limit reached|1760000000", "resets 2025-10-09 08:53 UTC"},
		{"Claude AI usage limit reached", "Claude AI usage limit reached"},
		{"You've hit your session limit", "You've hit your session limit"},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := classifyClaudeCodeFailure(tt.msg)
			if !errors.Is(err, ErrFatalForRun) || !errors.Is(err, ErrClaudeCodeUsageLimit) {
				t.Fatalf("err = %v, want the run-fatal usage limit", err)
			}
			if want := ErrClaudeCodeUsageLimit.Error() + ": " + tt.detail; FatalCause(err).Error() != want {
				t.Errorf("FatalCause() = %q, want %q", FatalCause(err), want)
			}
		})
	}
}
