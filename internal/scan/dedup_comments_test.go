// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package scan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
)

type cannedDedupClient struct {
	reply string
	err   error
	got   llm.ChatRequest
}

func (c *cannedDedupClient) CompletionsWithCtx(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.got = req
	if c.err != nil {
		return nil, c.err
	}
	content := c.reply
	return &llm.ChatResponse{
		Choices: []llm.Choice{{Message: llm.ResponseMessage{Content: &content}}},
		Usage:   &llm.UsageInfo{TotalTokens: 7},
	}, nil
}

func dedupFixture() ([]model.LlmComment, *template.LlmConversation) {
	comments := []model.LlmComment{
		{Path: "extract.ts", Content: "componentBody runs twice", ExistingCode: "componentBody(a)"},
		{Path: "extract.ts", Content: "componentBody is resolved twice here too", ExistingCode: "componentBody(a)"},
		{Path: "compile.ts", Content: "`in` walks the prototype chain"},
	}
	conv := &template.LlmConversation{Messages: []template.ChatMessage{
		{Role: "system", Content: "dedup"},
		{Role: "user", Content: "<batch_comments>{{batch_comments}}</batch_comments>"},
	}}
	return comments, conv
}

func TestDedupCommentsMergesGroups(t *testing.T) {
	comments, conv := dedupFixture()
	client := &cannedDedupClient{reply: `{"groups":[{"members":["c-0","c-1"],"merged_content":"componentBody is resolved twice"},{"members":["c-2"]}]}`}
	out, usage, err := DedupComments(context.Background(), client, "m", conv, comments, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Content != "componentBody is resolved twice" || out[1].Path != "compile.ts" {
		t.Fatalf("out = %+v", out)
	}
	if usage == nil || usage.TotalTokens != 7 {
		t.Errorf("usage = %+v", usage)
	}
	if !strings.Contains(client.got.Messages[1].ExtractText(), `"id":"c-2"`) {
		t.Errorf("comments were not rendered into the prompt: %q", client.got.Messages[1].ExtractText())
	}
}

func TestDedupCommentsKeepsOriginalsOnBadOutput(t *testing.T) {
	comments, conv := dedupFixture()
	for name, client := range map[string]*cannedDedupClient{
		"malformed":       {reply: "not json"},
		"drops a comment": {reply: `{"groups":[{"members":["c-0","c-1"]}]}`},
		"llm error":       {err: errors.New("boom")},
	} {
		out, _, err := DedupComments(context.Background(), client, "m", conv, comments, 1000)
		if err == nil {
			t.Errorf("%s: expected an error", name)
		}
		if len(out) != len(comments) {
			t.Errorf("%s: originals must survive, got %d of %d", name, len(out), len(comments))
		}
	}
}
