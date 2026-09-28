// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readAllFakeDumps(t *testing.T, dump string) []fakeClaudeDump {
	t.Helper()
	data, err := os.ReadFile(dump + ".all")
	if err != nil {
		t.Fatal(err)
	}
	var out []fakeClaudeDump
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var d fakeClaudeDump
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

// loopRequests mimics OCR's main loop: the second request repeats the first,
// then appends the model's tool calls and their results.
func loopRequests() (ChatRequest, ChatRequest) {
	first := ChatRequest{
		SessionID: "group-a",
		Messages:  []Message{NewTextMessage("system", "Review."), NewTextMessage("user", "the full diff")},
		Tools:     testTools(),
	}
	second := first
	second.Messages = append(append([]Message{}, first.Messages...),
		NewToolCallMessage("", []ToolCall{{ID: "call_0", Type: "function", Function: FunctionCall{Name: "code_comment", Arguments: `{"content":"x"}`}}}, NativeTurn{}, ""),
		NewToolResultMessage("call_0", "comment recorded"),
	)
	return first, second
}

func TestClaudeCodeClientResumesLoopConversation(t *testing.T) {
	dump := useFakeClaude(t, "tools")
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku"})
	t.Cleanup(func() { _ = c.Close() })
	first, second := loopRequests()
	for _, req := range []ChatRequest{first, second} {
		if _, err := c.CompletionsWithCtx(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}

	calls := readAllFakeDumps(t, dump)
	sid, ok := argValue(t, calls[0].Args, "--session-id")
	if !ok || sid == "" {
		t.Fatalf("first call must pin a session id: %q", calls[0].Args)
	}
	if containsString(calls[0].Args, "--no-session-persistence") {
		t.Error("a resumable conversation must be persisted")
	}
	if got, _ := argValue(t, calls[1].Args, "--resume"); got != sid {
		t.Errorf("second call --resume = %q, want %q", got, sid)
	}
	if strings.Contains(calls[1].Stdin, "the full diff") {
		t.Error("a resumed call must send only the new messages")
	}
	if !strings.Contains(calls[1].Stdin, `tool_call_id="call_0" name="code_comment"`) || !strings.Contains(calls[1].Stdin, "comment recorded") {
		t.Errorf("resumed stdin = %q", calls[1].Stdin)
	}
	if calls[0].Dir != calls[1].Dir {
		t.Error("resume only finds a session from the same working directory")
	}
}

func TestClaudeCodeClientStartsOverWhenHistoryIsRewritten(t *testing.T) {
	dump := useFakeClaude(t, "tools")
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku"})
	t.Cleanup(func() { _ = c.Close() })
	first, second := loopRequests()
	// Memory compression rewrites earlier turns; the stored session no longer
	// matches, so the full transcript must be sent to a fresh session.
	second.Messages[1] = NewTextMessage("user", "compressed summary")
	for _, req := range []ChatRequest{first, second} {
		if _, err := c.CompletionsWithCtx(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	calls := readAllFakeDumps(t, dump)
	if _, ok := argValue(t, calls[1].Args, "--resume"); ok {
		t.Error("rewritten history must not be resumed")
	}
	if _, ok := argValue(t, calls[1].Args, "--session-id"); !ok || !strings.Contains(calls[1].Stdin, "compressed summary") {
		t.Errorf("expected a fresh full session, got args %q stdin %q", calls[1].Args, calls[1].Stdin)
	}
}

func TestClaudeCodeClientFallsBackWhenResumeFails(t *testing.T) {
	dump := useFakeClaude(t, "resume-fails")
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku"})
	t.Cleanup(func() { _ = c.Close() })
	first, second := loopRequests()
	for _, req := range []ChatRequest{first, second} {
		if _, err := c.CompletionsWithCtx(context.Background(), req); err != nil {
			t.Fatalf("resume failure must fall back to a full call: %v", err)
		}
	}
	calls := readAllFakeDumps(t, dump)
	if len(calls) != 3 {
		t.Fatalf("want first, failed resume, full retry = 3 calls, got %d", len(calls))
	}
	if _, ok := argValue(t, calls[2].Args, "--session-id"); !ok || !strings.Contains(calls[2].Stdin, "the full diff") {
		t.Errorf("retry must be a fresh full session: %q", calls[2].Args)
	}
}

func TestClaudeCodeClientWithoutSessionIDStaysStateless(t *testing.T) {
	dump := useFakeClaude(t, "tools")
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku"})
	if _, err := c.CompletionsWithCtx(context.Background(), toolRequest()); err != nil {
		t.Fatal(err)
	}
	d := readFakeDump(t, dump)
	if !containsString(d.Args, "--no-session-persistence") {
		t.Error("one-shot requests must not persist a session")
	}
	if _, ok := argValue(t, d.Args, "--session-id"); ok {
		t.Error("one-shot requests must not pin a session")
	}
}

func TestClaudeCodeClientCloseRemovesSessions(t *testing.T) {
	useFakeClaude(t, "tools")
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	c := NewClaudeCodeClient(ClientConfig{Model: "haiku"})
	first, _ := loopRequests()
	if _, err := c.CompletionsWithCtx(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	transcripts, _ := filepath.Glob(filepath.Join(cfg, "projects", "*", "*.jsonl"))
	if len(transcripts) != 1 {
		t.Fatalf("fake CLI should have left one transcript, found %d", len(transcripts))
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(transcripts[0]); !os.IsNotExist(err) {
		t.Error("Close must delete the session transcript")
	}
	if _, err := os.Stat(filepath.Dir(transcripts[0])); !os.IsNotExist(err) {
		t.Error("Close must remove the emptied project directory")
	}
}
