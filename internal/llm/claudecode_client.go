// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"errors"
	"time"
)

// ClaudeCodeClient serves chat requests by running the local Claude Code CLI
// in headless mode, one process per request.
type ClaudeCodeClient struct {
	bin     string
	model   string
	timeout time.Duration
}

// NewClaudeCodeClient builds a client from the resolved endpoint config. URL,
// key and headers are ignored: the CLI authenticates with its own login.
func NewClaudeCodeClient(cfg ClientConfig) *ClaudeCodeClient {
	return &ClaudeCodeClient{model: cfg.Model, timeout: cfg.Timeout}
}

// CompletionsWithCtx implements LLMClient.
func (c *ClaudeCodeClient) CompletionsWithCtx(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return nil, errors.New("claude-code: not implemented")
}
