// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// envClaudeCodeBin overrides the claude executable; otherwise it is looked up
// on PATH.
const envClaudeCodeBin = "OCR_CLAUDE_CODE_BIN"

// claudeCodeScrubbedEnv lists variables that would make Claude Code bill an
// API account or route to another gateway instead of using its own login,
// which is the whole reason to pick this provider.
var claudeCodeScrubbedEnv = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
}

// ClaudeCodeClient serves chat requests by running the local Claude Code CLI
// in headless mode, one process per request.
type ClaudeCodeClient struct {
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
	inv, err := buildClaudeCodeInvocation(req, c.model)
	if err != nil {
		return nil, err
	}
	bin, err := claudeCodeBinary()
	if err != nil {
		return nil, err
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// An empty scratch directory keeps project CLAUDE.md / AGENTS.md files out
	// of the model's context; OCR already supplies all the context it wants.
	dir, err := os.MkdirTemp("", "ocr-claude-*")
	if err != nil {
		return nil, fmt.Errorf("claude-code: create working directory: %w", err)
	}
	defer os.RemoveAll(dir)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, inv.Args...)
	cmd.Dir = dir
	cmd.Env = claudeCodeEnv(os.Environ())
	cmd.Stdin = strings.NewReader(inv.Stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = 5 * time.Second
	setClaudeCodeProcAttr(cmd)

	runErr := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("claude-code: %w", ctxErr)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	resp, parseErr := parseClaudeCodeResult(stdout.Bytes(), inv.Structured, model)
	switch {
	case runErr == nil:
		return resp, parseErr
	case parseErr != nil && !errors.Is(parseErr, errClaudeCodeUnparsable):
		// The CLI printed a proper error result; its message beats "exit status 1".
		return nil, parseErr
	default:
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = stdout.String()
		}
		return nil, fmt.Errorf("%w (%v)", classifyClaudeCodeFailure(detail), runErr)
	}
}

func claudeCodeBinary() (string, error) {
	if bin := strings.TrimSpace(os.Getenv(envClaudeCodeBin)); bin != "" {
		if _, err := os.Stat(bin); err != nil {
			return "", fmt.Errorf("claude-code: %s=%q: %w", envClaudeCodeBin, bin, err)
		}
		return bin, nil
	}
	bin, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude-code: claude CLI not found on PATH (install Claude Code or set %s): %w", envClaudeCodeBin, err)
	}
	return bin, nil
}

func claudeCodeEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		scrub := false
		for _, s := range claudeCodeScrubbedEnv {
			if key == s {
				scrub = true
				break
			}
		}
		if !scrub {
			out = append(out, kv)
		}
	}
	return out
}
