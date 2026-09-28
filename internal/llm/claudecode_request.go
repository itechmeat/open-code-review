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
You are the model inside a tool-calling agent loop, and each reply is one step of that loop.
- StructuredOutput is the only tool you can invoke, and you invoke it exactly once per step. The agent's tools named in these instructions are not directly callable; request them by listing them in the "tool_calls" field of StructuredOutput, each with the tool "name" and its "arguments" object. Several independent calls per step are allowed.
- Tool results arrive in the next turn as <message role="tool" tool_call_id="..."> elements. When you need information, request it and wait for the next turn instead of guessing.
- Results the instructions ask you to report through a tool (findings, comments, answers) exist only when that tool is called. Text in "content" is not delivered to anyone as a result; keep it to a short note, or leave it empty.
- Call a finishing tool only after every result has been reported through its tool.`

// envClaudeCodeEffort selects Claude Code's reasoning effort. Thinking time
// dominates each call's latency (measured on a real OCR main task: sonnet
// ~12 s at low, ~27 s at medium, 25-60 s unset), and OCR's plan-and-review
// pipeline already structures the work, so low is the default. "auto" leaves
// the choice to Claude Code.
const envClaudeCodeEffort = "OCR_CLAUDE_CODE_EFFORT"

const claudeCodeDefaultEffort = "low"

var claudeCodeEfforts = []string{"low", "medium", "high", "xhigh", "max"}

// claudeCodeDefaultSystemPrompt stands in for an empty system prompt: an empty
// --system-prompt makes the CLI fall back to its own coding-agent prompt.
const claudeCodeDefaultSystemPrompt = "Follow the user's instructions precisely."

type claudeCodeInvocation struct {
	// Args excludes the system prompt, which the client writes to a file and
	// passes with --system-prompt-file.
	Args         []string
	SystemPrompt string
	Stdin        string
	// Structured is true when the answer is expected in structured_output
	// (tool calls) rather than in the plain result text.
	Structured bool
}

func buildClaudeCodeInvocation(req ChatRequest, defaultModel, effort string) (claudeCodeInvocation, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(defaultModel)
	}
	if model == "" {
		return claudeCodeInvocation{}, errors.New("claude-code: no model configured")
	}

	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		effort = claudeCodeDefaultEffort
	}
	if effort != "auto" && !containsEffort(effort) {
		return claudeCodeInvocation{}, fmt.Errorf("claude-code: %s=%q; want auto or one of %s", envClaudeCodeEffort, effort, strings.Join(claudeCodeEfforts, ", "))
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

	// Everything that can be large travels on stdin, and the system prompt in a
	// file: a Windows .cmd shim would cut an argument at its first newline, and
	// argv is visible to other local users.
	args := []string{
		"-p",
		"--output-format", "json",
		"--model", model,
		"--tools", "",
		"--strict-mcp-config",
		"--setting-sources", "",
	}
	if effort != "auto" {
		args = append(args, "--effort", effort)
	}
	if structured {
		// OCR never sets tool_choice, yet every tool-bearing request it makes
		// expects an action, so only an explicit "auto" allows an empty list.
		schema, err := json.Marshal(claudeCodeSchema(req.Tools, req.ToolChoice != "auto"))
		if err != nil {
			return claudeCodeInvocation{}, fmt.Errorf("claude-code: encode tool schema: %w", err)
		}
		args = append(args, "--json-schema", string(schema))
	}

	return claudeCodeInvocation{
		Args:         args,
		SystemPrompt: systemPrompt,
		Stdin:        renderClaudeCodeTranscript(conversation),
		Structured:   structured,
	}, nil
}

func containsEffort(effort string) bool {
	for _, e := range claudeCodeEfforts {
		if e == effort {
			return true
		}
	}
	return false
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
	return renderClaudeCodeMessages(msgs, 0, false)
}

// renderClaudeCodeDelta renders what a resumed session has not seen yet: the
// messages from index from on, minus assistant turns, which the session
// already holds in its own form.
func renderClaudeCodeDelta(msgs []Message, from int) string {
	return renderClaudeCodeMessages(msgs, from, true)
}

func renderClaudeCodeMessages(msgs []Message, from int, skipAssistant bool) string {
	// Tool results carry the tool name as well as the call id, so a resumed
	// session whose assistant turns are not replayed can still pair them.
	names := make(map[string]string)
	for i := range msgs {
		for _, tc := range msgs[i].ToolCalls {
			names[tc.ID] = tc.Function.Name
		}
	}
	var sb strings.Builder
	for i := from; i < len(msgs); i++ {
		m := &msgs[i]
		if m.Role == "system" || (skipAssistant && m.Role == "assistant") {
			continue
		}
		if m.Role == "tool" {
			fmt.Fprintf(&sb, "<message role=\"tool\" tool_call_id=%q", m.ToolCallID)
			if name := names[m.ToolCallID]; name != "" {
				fmt.Fprintf(&sb, " name=%q", name)
			}
			sb.WriteString(">\n")
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
