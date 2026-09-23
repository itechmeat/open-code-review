// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func argValue(t *testing.T, args []string, flag string) (string, bool) {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Fatalf("flag %s has no value in %q", flag, args)
			}
			return args[i+1], true
		}
	}
	return "", false
}

func testTools() []ToolDef {
	return []ToolDef{
		{Type: "function", Function: FunctionDef{
			Name:        "code_comment",
			Description: "Report an issue.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"content": map[string]any{"type": "string"}},
				"required":   []any{"content"},
			},
		}},
		{Type: "function", Function: FunctionDef{Name: "task_done", Description: "Finish."}},
	}
}

func TestBuildClaudeCodeInvocationTextMode(t *testing.T) {
	inv, err := buildClaudeCodeInvocation(ChatRequest{
		Messages: []Message{
			NewTextMessage("system", "Group the files."),
			NewTextMessage("user", "a.go\nb.go"),
		},
	}, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Structured {
		t.Fatal("text request must not be structured")
	}
	if _, ok := argValue(t, inv.Args, "--json-schema"); ok {
		t.Fatal("text request must not pass --json-schema")
	}
	for _, flag := range []string{"-p", "--strict-mcp-config", "--no-session-persistence"} {
		if !containsString(inv.Args, flag) {
			t.Errorf("missing %s in %q", flag, inv.Args)
		}
	}
	for flag, want := range map[string]string{
		"--output-format":   "json",
		"--model":           "haiku",
		"--tools":           "",
		"--setting-sources": "",
	} {
		if got, ok := argValue(t, inv.Args, flag); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", flag, got, ok, want)
		}
	}
	if inv.SystemPrompt != "Group the files." {
		t.Errorf("SystemPrompt = %q", inv.SystemPrompt)
	}
	// The prompt travels in a file: a cmd.exe shim would cut an argument at its
	// first newline, and argv is visible to other local users.
	if _, ok := argValue(t, inv.Args, "--system-prompt"); ok {
		t.Error("system prompt must not be passed in argv")
	}
	if strings.Contains(inv.Stdin, "Group the files.") {
		t.Error("system prompt leaked into stdin")
	}
	if !strings.Contains(inv.Stdin, "a.go\nb.go") {
		t.Errorf("stdin = %q", inv.Stdin)
	}
}

func TestBuildClaudeCodeInvocationStructuredMode(t *testing.T) {
	inv, err := buildClaudeCodeInvocation(ChatRequest{
		Model:    "opus",
		Messages: []Message{NewTextMessage("system", "Review."), NewTextMessage("user", "diff")},
		Tools:    testTools(),
	}, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Structured {
		t.Fatal("tool request must be structured")
	}
	if got, _ := argValue(t, inv.Args, "--model"); got != "opus" {
		t.Errorf("request model must win, got %q", got)
	}
	sys := inv.SystemPrompt
	if !strings.HasPrefix(sys, "Review.") || !strings.Contains(sys, "tool_calls") {
		t.Errorf("system prompt must keep OCR's prompt and add the bridge instruction, got %q", sys)
	}
	// A live haiku run wrote its findings into content and only called
	// task_done; the bridge must say content is never delivered as a result.
	for _, phrase := range []string{"one step", "not delivered", "next turn"} {
		if !strings.Contains(sys, phrase) {
			t.Errorf("bridge instruction must mention %q, got %q", phrase, sys)
		}
	}
	raw, ok := argValue(t, inv.Args, "--json-schema")
	if !ok {
		t.Fatal("missing --json-schema")
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	branches := schema["properties"].(map[string]any)["tool_calls"].(map[string]any)["items"].(map[string]any)["anyOf"].([]any)
	if len(branches) != 2 {
		t.Fatalf("want 2 branches, got %d", len(branches))
	}
	first := branches[0].(map[string]any)
	if first["description"] != "Report an issue." {
		t.Errorf("branch description = %v", first["description"])
	}
	props := first["properties"].(map[string]any)
	if props["name"].(map[string]any)["const"] != "code_comment" {
		t.Errorf("branch name const = %v", props["name"])
	}
	if props["arguments"].(map[string]any)["required"].([]any)[0] != "content" {
		t.Errorf("arguments must carry the tool's own schema, got %v", props["arguments"])
	}
	second := branches[1].(map[string]any)["properties"].(map[string]any)["arguments"].(map[string]any)
	if second["type"] != "object" {
		t.Errorf("tool without parameters must get an object schema, got %v", second)
	}
}

func TestClaudeCodeSchemaToolChoice(t *testing.T) {
	minItems := func(choice string) (any, bool) {
		t.Helper()
		inv, err := buildClaudeCodeInvocation(ChatRequest{
			Messages:   []Message{NewTextMessage("user", "x")},
			Tools:      testTools(),
			ToolChoice: choice,
		}, "haiku", "")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := argValue(t, inv.Args, "--json-schema")
		var schema map[string]any
		if err := json.Unmarshal([]byte(raw), &schema); err != nil {
			t.Fatal(err)
		}
		v, ok := schema["properties"].(map[string]any)["tool_calls"].(map[string]any)["minItems"]
		return v, ok
	}
	// Every OCR request that offers tools expects an action; a live filter
	// call answered with an empty list instead of approve_all_comments.
	for _, choice := range []string{"", "required"} {
		if v, ok := minItems(choice); !ok || v != float64(1) {
			t.Errorf("tool_choice %q: minItems = %v (present %v), want 1", choice, v, ok)
		}
	}
	if _, ok := minItems("auto"); ok {
		t.Error("explicit auto must allow an empty tool_calls")
	}

	inv, err := buildClaudeCodeInvocation(ChatRequest{
		Messages:   []Message{NewTextMessage("user", "x")},
		Tools:      testTools(),
		ToolChoice: "none",
	}, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Structured {
		t.Error("tool_choice none must disable structured output")
	}
}

func TestRenderClaudeCodeTranscript(t *testing.T) {
	msgs := []Message{
		NewTextMessage("system", "ignored"),
		NewTextMessage("user", "Review a.go"),
		NewToolCallMessage("Looking.", []ToolCall{{ID: "c1", Type: "function",
			Function: FunctionCall{Name: "file_read", Arguments: `{"path":"a.go"}`}}}, NativeTurn{}, ""),
		NewToolResultMessage("c1", "package a"),
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Go on."}}},
	}
	want := `<message role="user">
Review a.go
</message>
<message role="assistant">
Looking.
<tool_call id="c1" name="file_read">{"path":"a.go"}</tool_call>
</message>
<message role="tool" tool_call_id="c1">
package a
</message>
<message role="user">
Go on.
</message>
`
	if got := renderClaudeCodeTranscript(msgs); got != want {
		t.Errorf("transcript mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildClaudeCodeInvocationLargePromptGoesToStdin(t *testing.T) {
	big := strings.Repeat("+line of diff\n", 150_000)
	inv, err := buildClaudeCodeInvocation(ChatRequest{
		Messages: []Message{NewTextMessage("system", "s"), NewTextMessage("user", big)},
		Tools:    testTools(),
	}, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inv.Stdin, big) {
		t.Fatal("large user message must reach stdin intact")
	}
	for _, a := range inv.Args {
		if len(a) > 64*1024 {
			t.Fatalf("argument of %d bytes would risk ARG_MAX", len(a))
		}
	}
}

func TestBuildClaudeCodeInvocationRequiresModel(t *testing.T) {
	_, err := buildClaudeCodeInvocation(ChatRequest{Messages: []Message{NewTextMessage("user", "x")}}, "", "")
	if err == nil || !strings.Contains(err.Error(), "no model") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildClaudeCodeInvocationDefaultsEmptySystemPrompt(t *testing.T) {
	inv, err := buildClaudeCodeInvocation(ChatRequest{Messages: []Message{NewTextMessage("user", "x")}}, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(inv.SystemPrompt) == "" {
		t.Error("an empty --system-prompt would fall back to Claude Code's own prompt")
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestBuildClaudeCodeInvocationEffort(t *testing.T) {
	req := ChatRequest{Messages: []Message{NewTextMessage("user", "x")}}
	tests := []struct {
		effort  string
		want    string
		present bool
	}{
		{"", "low", true},
		{"high", "high", true},
		{" XHigh ", "xhigh", true},
		{"auto", "", false},
	}
	for _, tt := range tests {
		inv, err := buildClaudeCodeInvocation(req, "haiku", tt.effort)
		if err != nil {
			t.Fatalf("effort %q: %v", tt.effort, err)
		}
		got, ok := argValue(t, inv.Args, "--effort")
		if ok != tt.present || got != tt.want {
			t.Errorf("effort %q: --effort = %q (present %v), want %q (present %v)", tt.effort, got, ok, tt.want, tt.present)
		}
	}
	if _, err := buildClaudeCodeInvocation(req, "haiku", "turbo"); err == nil || !strings.Contains(err.Error(), envClaudeCodeEffort) {
		t.Fatalf("unknown effort must name %s, got %v", envClaudeCodeEffort, err)
	}
}
