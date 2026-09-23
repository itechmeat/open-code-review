// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"testing"

	"github.com/alibaba/open-code-review/internal/llm"
)

func TestRecordExtraUsageCountsTowardTotals(t *testing.T) {
	a := New(Args{})
	before := a.TotalTokensUsed()
	a.RecordExtraUsage(&llm.UsageInfo{TotalTokens: 30, PromptTokens: 20, CompletionTokens: 10})
	a.RecordExtraUsage(nil)
	if got := a.TotalTokensUsed() - before; got != 30 {
		t.Errorf("total grew by %d, want 30", got)
	}
}
