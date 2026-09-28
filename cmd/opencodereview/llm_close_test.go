// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"context"
	"testing"

	"github.com/alibaba/open-code-review/internal/llm"
)

type closingClient struct{ closed bool }

func (c *closingClient) CompletionsWithCtx(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}
func (c *closingClient) Close() error { c.closed = true; return nil }

type plainClient struct{}

func (plainClient) CompletionsWithCtx(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}

func TestCloseLLMClient(t *testing.T) {
	c := &closingClient{}
	closeLLMClient(c)
	if !c.closed {
		t.Error("a client with Close must be closed")
	}
	closeLLMClient(plainClient{}) // must not panic
	closeLLMClient(nil)
}
