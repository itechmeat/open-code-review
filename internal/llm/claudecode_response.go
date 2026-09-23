// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrClaudeCodeNotLoggedIn means the claude CLI has no usable login.
	ErrClaudeCodeNotLoggedIn = errors.New("claude-code: the claude CLI is not logged in (run `claude` and use /login)")
	// ErrClaudeCodeUsageLimit means the Claude plan's usage window is exhausted.
	ErrClaudeCodeUsageLimit = errors.New("claude-code: Claude usage limit reached (wait for the limit window or lower --concurrency)")

	errClaudeCodeUnparsable = errors.New("claude-code: unparsable CLI output")
)

// claudeCodeResult is the single JSON object `claude -p --output-format json`
// prints on completion.
type claudeCodeResult struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	Usage            struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

type claudeCodeStructured struct {
	Content   string `json:"content"`
	ToolCalls []struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"tool_calls"`
}

func parseClaudeCodeResult(stdout []byte, structured bool, model string) (*ChatResponse, error) {
	var res claudeCodeResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &res); err != nil {
		return nil, fmt.Errorf("%w %q: %v", errClaudeCodeUnparsable, truncateForError(string(stdout)), err)
	}
	if res.IsError {
		return nil, classifyClaudeCodeFailure(res.Result)
	}

	msg := ResponseMessage{Role: "assistant"}
	finish := "stop"
	if structured {
		if len(res.StructuredOutput) == 0 || string(res.StructuredOutput) == "null" {
			return nil, fmt.Errorf("claude-code: no structured output (subtype %q): %s", res.Subtype, truncateForError(res.Result))
		}
		var out claudeCodeStructured
		if err := json.Unmarshal(res.StructuredOutput, &out); err != nil {
			return nil, fmt.Errorf("claude-code: malformed structured output: %w", err)
		}
		if out.Content != "" {
			content := out.Content
			msg.Content = &content
		}
		for i, call := range out.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:       fmt.Sprintf("call_%d", i),
				Type:     "function",
				Function: FunctionCall{Name: call.Name, Arguments: compactToolArguments(call.Arguments)},
			})
		}
		if len(msg.ToolCalls) > 0 {
			finish = "tool_calls"
		}
	} else {
		content := res.Result
		msg.Content = &content
	}

	// Claude Code reports cached input separately; OCR's prompt count includes
	// it, as the Anthropic client's accounting does.
	prompt := res.Usage.InputTokens + res.Usage.CacheCreationInputTokens + res.Usage.CacheReadInputTokens
	return &ChatResponse{
		Model:   model,
		Choices: []Choice{{Message: msg, FinishReason: finish}},
		Usage: &UsageInfo{
			TotalTokens:      prompt + res.Usage.OutputTokens,
			PromptTokens:     prompt,
			CompletionTokens: res.Usage.OutputTokens,
			CacheReadTokens:  res.Usage.CacheReadInputTokens,
			CacheWriteTokens: res.Usage.CacheCreationInputTokens,
		},
	}, nil
}

// compactToolArguments normalizes arguments to a JSON object string. A model
// may omit arguments for a parameterless tool; OCR's tool parsers expect an
// object, so "null" would fail where "{}" works.
func compactToolArguments(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return string(trimmed)
	}
	return buf.String()
}

func classifyClaudeCodeFailure(message string) error {
	msg := strings.TrimSpace(message)
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not logged in"), strings.Contains(lower, "/login"), strings.Contains(lower, "invalid api key"):
		return fmt.Errorf("%w: %s", ErrClaudeCodeNotLoggedIn, truncateForError(msg))
	case strings.Contains(lower, "usage limit"), strings.Contains(lower, "hit your limit"), strings.Contains(lower, "rate limit"):
		return fmt.Errorf("%w: %s", ErrClaudeCodeUsageLimit, truncateForError(msg))
	default:
		return fmt.Errorf("claude-code: %s", truncateForError(msg))
	}
}

func truncateForError(s string) string {
	const limit = 2048
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return "…" + s[len(s)-limit:]
}
