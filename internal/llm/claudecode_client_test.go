// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// useFakeClaude routes the client to the test binary in the given mode and
// returns the path the fake dumps its argv, stdin and env to.
func useFakeClaude(t *testing.T, mode string) string {
	t.Helper()
	dump := filepath.Join(t.TempDir(), "dump.json")
	t.Setenv(envClaudeCodeBin, os.Args[0])
	t.Setenv("OCR_FAKE_CLAUDE", mode)
	t.Setenv("OCR_FAKE_CLAUDE_DUMP", dump)
	return dump
}

func readFakeDump(t *testing.T, path string) fakeClaudeDump {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fake claude did not run: %v", err)
	}
	var d fakeClaudeDump
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func toolRequest() ChatRequest {
	return ChatRequest{
		Messages: []Message{NewTextMessage("system", "Review."), NewTextMessage("user", "the diff")},
		Tools:    testTools(),
	}
}

func TestClaudeCodeClientMapsToolCalls(t *testing.T) {
	dump := useFakeClaude(t, "tools")
	c := NewClaudeCodeClient(ClientConfig{Model: "sonnet"})

	resp, err := c.CompletionsWithCtx(context.Background(), toolRequest())
	if err != nil {
		t.Fatal(err)
	}
	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 2 {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	first := msg.ToolCalls[0]
	if first.ID == "" || first.ID == msg.ToolCalls[1].ID || first.Type != "function" ||
		first.Function.Name != "code_comment" || first.Function.Arguments != `{"content":"x"}` {
		t.Errorf("first call = %+v", first)
	}
	if msg.Content == nil || *msg.Content != "checked" {
		t.Errorf("content = %v", msg.Content)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q", resp.Choices[0].FinishReason)
	}
	u := resp.Usage
	if u == nil || u.PromptTokens != 15 || u.CompletionTokens != 5 || u.CacheReadTokens != 2 ||
		u.CacheWriteTokens != 3 || u.TotalTokens != 20 {
		t.Errorf("usage = %+v", u)
	}
	if resp.Model != "sonnet" {
		t.Errorf("model = %q", resp.Model)
	}

	d := readFakeDump(t, dump)
	if !strings.Contains(d.Stdin, "the diff") {
		t.Errorf("stdin = %q", d.Stdin)
	}
	if got, _ := argValue(t, d.Args, "--model"); got != "sonnet" {
		t.Errorf("--model = %q", got)
	}
	if _, ok := argValue(t, d.Args, "--json-schema"); !ok {
		t.Error("structured request must pass --json-schema")
	}
	cwd, _ := os.Getwd()
	if d.Dir == cwd {
		t.Error("claude must not run in OCR's working directory, where project CLAUDE.md files live")
	}
	if _, err := os.Stat(d.Dir); !os.IsNotExist(err) {
		t.Errorf("temporary working directory %s was not removed", d.Dir)
	}
}

func TestClaudeCodeClientNullArgumentsBecomeEmptyObject(t *testing.T) {
	useFakeClaude(t, "null-args")
	resp, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(), toolRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Arguments != "{}" {
			t.Errorf("arguments = %q, want {}", tc.Function.Arguments)
		}
	}
	if resp.Choices[0].Message.Content != nil {
		t.Errorf("empty structured content must stay nil, got %q", *resp.Choices[0].Message.Content)
	}
}

func TestClaudeCodeClientTextMode(t *testing.T) {
	dump := useFakeClaude(t, "text")
	resp, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(),
		ChatRequest{Messages: []Message{NewTextMessage("user", "group these")}})
	if err != nil {
		t.Fatal(err)
	}
	msg := resp.Choices[0].Message
	if msg.Content == nil || *msg.Content != "hello" || len(msg.ToolCalls) != 0 || resp.Choices[0].FinishReason != "stop" {
		t.Errorf("response = %+v", resp.Choices[0])
	}
	if _, ok := argValue(t, readFakeDump(t, dump).Args, "--json-schema"); ok {
		t.Error("text request must not pass --json-schema")
	}
}

func TestClaudeCodeClientMissingStructuredOutput(t *testing.T) {
	useFakeClaude(t, "missing-structured")
	_, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(), toolRequest())
	if err == nil || !strings.Contains(err.Error(), "structured output") {
		t.Fatalf("err = %v", err)
	}
}

func TestClaudeCodeClientErrors(t *testing.T) {
	tests := []struct {
		mode     string
		sentinel error
		sub      string
	}{
		{"error-login", ErrClaudeCodeNotLoggedIn, "Not logged in"},
		{"error-limit", ErrClaudeCodeUsageLimit, "usage limit"},
		{"stderr-only", nil, "boom: unexpected failure"},
		{"garbage", nil, "not json"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			useFakeClaude(t, tt.mode)
			_, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(), toolRequest())
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("err = %v, want wrapping %v", err, tt.sentinel)
			}
			if !strings.Contains(err.Error(), tt.sub) {
				t.Errorf("err = %v, want substring %q", err, tt.sub)
			}
		})
	}
}

func TestClaudeCodeClientCancelKillsProcess(t *testing.T) {
	useFakeClaude(t, "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	_, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(ctx, toolRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancel took %v", elapsed)
	}
}

func TestClaudeCodeClientTimeout(t *testing.T) {
	useFakeClaude(t, "sleep")
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku", Timeout: 200 * time.Millisecond})
	_, err := c.CompletionsWithCtx(context.Background(), toolRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestClaudeCodeClientScrubsBillingEnv(t *testing.T) {
	dump := useFakeClaude(t, "tools")
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-not-leak")
	t.Setenv("ANTHROPIC_BASE_URL", "https://proxy.example")
	if _, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(), toolRequest()); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(readFakeDump(t, dump).Env, "\n")
	if strings.Contains(env, "sk-should-not-leak") || strings.Contains(env, "proxy.example") {
		t.Error("billing/routing env reached the claude process")
	}
	if !strings.Contains(env, "OCR_FAKE_CLAUDE=tools") {
		t.Error("ordinary env must pass through")
	}
}

func TestClaudeCodeEnv(t *testing.T) {
	got := claudeCodeEnv([]string{
		"PATH=/bin", "ANTHROPIC_API_KEY=k", "ANTHROPIC_AUTH_TOKEN=t", "ANTHROPIC_BASE_URL=u",
		"CLAUDE_CODE_USE_BEDROCK=1", "CLAUDE_CODE_USE_VERTEX=1", "ANTHROPIC_API_KEY_HELPER=keep",
	})
	want := []string{"PATH=/bin", "ANTHROPIC_API_KEY_HELPER=keep"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("env = %q, want %q", got, want)
	}
}

func TestClaudeCodeClientMissingBinary(t *testing.T) {
	t.Setenv(envClaudeCodeBin, filepath.Join(t.TempDir(), "no-such-claude"))
	_, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(context.Background(), toolRequest())
	if err == nil || !strings.Contains(err.Error(), envClaudeCodeBin) {
		t.Fatalf("err = %v, want a hint about %s", err, envClaudeCodeBin)
	}
}

func TestClaudeCodeClientRejectsMissingModel(t *testing.T) {
	useFakeClaude(t, "tools")
	if _, err := NewClaudeCodeClient(ClientConfig{}).CompletionsWithCtx(context.Background(), toolRequest()); err == nil {
		t.Fatal("expected error without a model")
	}
}

func TestClaudeCodeBinaryFromPATH(t *testing.T) {
	dir := t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		name = "claude.exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envClaudeCodeBin, "")
	t.Setenv("PATH", dir)
	got, err := claudeCodeBinary()
	if err != nil || filepath.Dir(got) != dir {
		t.Fatalf("claudeCodeBinary() = %q, %v", got, err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := claudeCodeBinary(); err == nil || !strings.Contains(err.Error(), "not found on PATH") {
		t.Fatalf("err = %v", err)
	}
}

func TestTruncateForErrorKeepsTail(t *testing.T) {
	long := strings.Repeat("a", 5000) + "the real cause"
	got := truncateForError(long)
	if len(got) > 2100 || !strings.HasSuffix(got, "the real cause") {
		t.Errorf("truncateForError kept %d bytes, suffix %q", len(got), got[len(got)-20:])
	}
}

func TestCompactToolArgumentsPassesThroughInvalidJSON(t *testing.T) {
	if got := compactToolArguments(json.RawMessage(`{"a": 1}`)); got != `{"a":1}` {
		t.Errorf("compact = %q", got)
	}
	if got := compactToolArguments(json.RawMessage(`{broken`)); got != `{broken` {
		t.Errorf("invalid JSON must pass through for OCR's argument repair, got %q", got)
	}
}
