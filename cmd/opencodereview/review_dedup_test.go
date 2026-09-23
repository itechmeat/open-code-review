// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
)

type dedupReplyClient struct{ calls int }

func (c *dedupReplyClient) CompletionsWithCtx(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls++
	reply := `{"groups":[{"members":["c-0","c-1"]},{"members":["c-2"]},{"members":["c-3"]}]}`
	return &llm.ChatResponse{Choices: []llm.Choice{{Message: llm.ResponseMessage{Content: &reply}}}, Usage: &llm.UsageInfo{TotalTokens: 9}}, nil
}

func reviewComments(n int) []model.LlmComment {
	out := make([]model.LlmComment, n)
	for i := range out {
		out[i] = model.LlmComment{Path: "a.ts", Content: fmt.Sprintf("finding %d", i)}
	}
	return out
}

func TestDedupReviewComments(t *testing.T) {
	client := &dedupReplyClient{}
	got, _ := dedupReviewComments(context.Background(), client, "m", 1000, reviewComments(4), false)
	if len(got) != 3 || client.calls != 1 {
		t.Fatalf("got %d comments after %d calls, want 3 after 1", len(got), client.calls)
	}
	if _, usage := dedupReviewComments(context.Background(), &dedupReplyClient{}, "m", 1000, reviewComments(4), false); usage == nil || usage.TotalTokens != 9 {
		t.Errorf("dedup usage must be returned for the run totals, got %+v", usage)
	}

	client = &dedupReplyClient{}
	if got, _ := dedupReviewComments(context.Background(), client, "m", 1000, reviewComments(4), true); len(got) != 4 || client.calls != 0 {
		t.Errorf("--no-dedup must skip the call: %d comments, %d calls", len(got), client.calls)
	}
	if got, _ := dedupReviewComments(context.Background(), client, "m", 1000, reviewComments(2), false); len(got) != 2 || client.calls != 0 {
		t.Errorf("below the minimum must skip the call: %d comments, %d calls", len(got), client.calls)
	}
}

func TestReviewHasNoDedupFlag(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"review"})
	if err != nil || cmd.Flags().Lookup("no-dedup") == nil {
		t.Fatal("review has no --no-dedup flag")
	}
}
