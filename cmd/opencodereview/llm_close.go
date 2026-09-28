// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import "github.com/alibaba/open-code-review/internal/llm"

// closeLLMClient releases what a client holds past a single request, such as
// the claude-code provider's persisted sessions. Clients without state have
// no Close and are left alone.
func closeLLMClient(c llm.LLMClient) {
	if closer, ok := c.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}
