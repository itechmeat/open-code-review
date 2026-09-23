// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import "github.com/alibaba/open-code-review/internal/llm"

// RecordExtraUsage adds tokens spent by LLM calls made after Run on the run's
// behalf, such as merging duplicate findings, to the totals the run reports.
func (a *Agent) RecordExtraUsage(u *llm.UsageInfo) {
	if u != nil {
		a.runner.RecordUsage(u)
	}
}
