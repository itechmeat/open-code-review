// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// claudeCodeBridgeInstruction is appended to OCR's system prompt whenever
// tools are offered. Claude Code only sees one StructuredOutput tool, so the
// model has to be told that OCR's tools are invoked through that output and
// how their results come back in the transcript.
const claudeCodeBridgeInstruction = `## Tool protocol
You are the model behind a tool-calling agent. Call tools by listing them in "tool_calls" of your structured output, each with the tool "name" and its "arguments" object; several calls per turn are allowed. Tool results arrive in the next turn as <message role="tool" tool_call_id="..."> elements. Put any plain-text reply in "content".`

// claudeCodeDefaultSystemPrompt stands in for an empty system prompt: an empty
// --system-prompt makes the CLI fall back to its own coding-agent prompt.
const claudeCodeDefaultSystemPrompt = "Follow the user's instructions precisely."

type claudeCodeInvocation struct {
	Args  []string
	Stdin string
	// Structured is true when the answer is expected in structured_output
	// (tool calls) rather than in the plain result text.
	Structured bool
}

func buildClaudeCodeInvocation(req ChatRequest, defaultModel string) (claudeCodeInvocation, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(defaultModel)
	}
	if model == "" {
		return claudeCodeInvocation{}, errors.New("claude-code: no model configured")
	}

	var system []string
	var conversation []Message
	for i := range req.Messages {
		if req.Messages[i].Role == "system" {
			if text := req.Messages[i].ExtractText(); text != "" {
				system = append(system, text)
			}
			continue
		}
		conversation = append(conversation, req.Messages[i])
	}

	structured := len(req.Tools) > 0 && req.ToolChoice != "none"
	systemPrompt := strings.Join(system, "\n\n")
	if structured {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + claudeCodeBridgeInstruction)
	}
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = claudeCodeDefaultSystemPrompt
	}

	// Everything that can be large travels on stdin; argv only carries the
	// system prompt and the schema, which are bounded by OCR's templates.
	args := []string{
		"-p",
		"--output-format", "json",
		"--model", model,
		"--tools", "",
		"--strict-mcp-config",
		"--setting-sources", "",
		"--no-session-persistence",
		"--system-prompt", systemPrompt,
	}
	if structured {
		schema, err := json.Marshal(claudeCodeSchema(req.Tools, req.ToolChoice == "required"))
		if err != nil {
			return claudeCodeInvocation{}, fmt.Errorf("claude-code: encode tool schema: %w", err)
		}
		args = append(args, "--json-schema", string(schema))
	}

	return claudeCodeInvocation{
		Args:       args,
		Stdin:      renderClaudeCodeTranscript(conversation),
		Structured: structured,
	}, nil
}

// claudeCodeSchema turns OCR's tool definitions into one structured-output
// schema. Each tool is an anyOf branch pinned by a const name, so the CLI's
// schema validation rejects unknown tools and malformed arguments before OCR
// ever sees them.
func claudeCodeSchema(tools []ToolDef, required bool) map[string]any {
	branches := make([]any, 0, len(tools))
	for _, t := range tools {
		params := t.Function.Parameters
		if len(params) == 0 {
			params = map[string]any{"type": "object"}
		}
		branches = append(branches, map[string]any{
			"type":        "object",
			"description": t.Function.Description,
			"properties": map[string]any{
				"name":      map[string]any{"const": t.Function.Name},
				"arguments": params,
			},
			"required": []any{"name", "arguments"},
		})
	}
	calls := map[string]any{
		"type":  "array",
		"items": map[string]any{"anyOf": branches},
	}
	if required {
		calls["minItems"] = 1
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content":    map[string]any{"type": "string"},
			"tool_calls": calls,
		},
		"required": []any{"tool_calls"},
	}
}

func renderClaudeCodeTranscript(msgs []Message) string {
	var sb strings.Builder
	for i := range msgs {
		m := &msgs[i]
		if m.Role == "system" {
			continue
		}
		if m.Role == "tool" {
			fmt.Fprintf(&sb, "<message role=\"tool\" tool_call_id=%q>\n", m.ToolCallID)
		} else {
			fmt.Fprintf(&sb, "<message role=%q>\n", m.Role)
		}
		if text := m.ExtractText(); text != "" {
			sb.WriteString(text)
			sb.WriteString("\n")
		}
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&sb, "<tool_call id=%q name=%q>%s</tool_call>\n", tc.ID, tc.Function.Name, tc.Function.Arguments)
		}
		sb.WriteString("</message>\n")
	}
	return sb.String()
}
