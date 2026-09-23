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
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// envClaudeCodeBin overrides the claude executable; otherwise it is looked up
// on PATH.
const envClaudeCodeBin = "OCR_CLAUDE_CODE_BIN"

// claudeCodeScrubbedEnv lists variables that would make Claude Code bill an
// API account, route to another gateway or remap the requested model instead
// of using its own login, plus the parent Claude Code session's identity so
// each run is an independent session even when OCR is launched from one.
var claudeCodeScrubbedEnv = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_CUSTOM_HEADERS",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
	"CLAUDE_CODE_USE_FOUNDRY",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"CLAUDECODE",
	"CLAUDE_CODE_SESSION_ID",
	"CLAUDE_CODE_CHILD_SESSION",
	"CLAUDE_CODE_MESSAGING_SOCKET",
	"CLAUDE_CODE_MESSAGING_TOKEN",
}

// claudeCodeDefaultTimeout bounds one request when the endpoint sets none.
// Plan, grouping and re-location calls run outside OCR's per-group timeout, so
// without it a hung CLI would hang the whole review.
const claudeCodeDefaultTimeout = 15 * time.Minute

// ClaudeCodeClient serves chat requests by running the local Claude Code CLI
// in headless mode, one process per request.
type ClaudeCodeClient struct {
	model   string
	effort  string
	timeout time.Duration

	mu       sync.Mutex
	workDir  string                       // stable cwd: --resume only finds sessions of the same directory
	threads  map[string]*claudeCodeThread // by ChatRequest.SessionID
	sessions []string                     // CLI session ids to delete on Close
}

// NewClaudeCodeClient builds a client from the resolved endpoint config. URL,
// key and headers are ignored: the CLI authenticates with its own login.
func NewClaudeCodeClient(cfg ClientConfig) *ClaudeCodeClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = claudeCodeDefaultTimeout
	}
	return &ClaudeCodeClient{model: cfg.Model, effort: os.Getenv(envClaudeCodeEffort), timeout: timeout}
}

// CompletionsWithCtx implements LLMClient.
func (c *ClaudeCodeClient) CompletionsWithCtx(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	inv, err := buildClaudeCodeInvocation(req, c.model, c.effort)
	if err != nil {
		return nil, err
	}
	bin, err := claudeCodeBinary()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if req.SessionID != "" {
		return c.completeInThread(ctx, bin, req, inv, model)
	}

	// An empty scratch directory keeps project CLAUDE.md / AGENTS.md files out
	// of the model's context; OCR already supplies all the context it wants.
	dir, err := os.MkdirTemp("", "ocr-claude-*")
	if err != nil {
		return nil, fmt.Errorf("claude-code: create working directory: %w", err)
	}
	defer os.RemoveAll(dir)
	return c.run(ctx, bin, dir, append(inv.Args, "--no-session-persistence"), inv.Stdin, inv, model)
}

// run executes one claude process in dir and maps its result.
func (c *ClaudeCodeClient) run(ctx context.Context, bin, dir string, args []string, stdin string,
	inv claudeCodeInvocation, model string) (*ChatResponse, error) {
	system, err := os.CreateTemp(dir, "system-prompt-*.txt")
	if err != nil {
		return nil, fmt.Errorf("claude-code: write system prompt: %w", err)
	}
	defer os.Remove(system.Name())
	_, werr := system.WriteString(inv.SystemPrompt)
	if cerr := system.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, fmt.Errorf("claude-code: write system prompt: %w", werr)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, append(args, "--system-prompt-file", system.Name())...)
	cmd.Dir = dir
	cmd.Env = claudeCodeEnv(os.Environ())
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = 5 * time.Second
	setClaudeCodeProcAttr(cmd)

	runErr := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("claude-code: %w", ctxErr)
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
		return nil, fmt.Errorf("%w (%s: %v)", classifyClaudeCodeFailure(detail), bin, runErr)
	}
}

// ClaudeCodeBinary reports the claude executable the claude-code provider
// runs, resolved the same way each request resolves it.
func ClaudeCodeBinary() (string, error) { return claudeCodeBinary() }

func claudeCodeBinary() (string, error) {
	bin := strings.TrimSpace(os.Getenv(envClaudeCodeBin))
	if bin != "" {
		if _, err := os.Stat(bin); err != nil {
			return "", fmt.Errorf("claude-code: %s=%q: %w", envClaudeCodeBin, bin, err)
		}
	} else {
		var err error
		if bin, err = exec.LookPath("claude"); err != nil {
			return "", fmt.Errorf("claude-code: claude CLI not found on PATH (install Claude Code or set %s): %w", envClaudeCodeBin, err)
		}
	}
	// cmd.exe re-parses a batch shim's arguments, mangling the JSON schema's
	// quotes, and killing the shim on cancel leaves the real CLI running.
	switch strings.ToLower(filepath.Ext(bin)) {
	case ".cmd", ".bat":
		return "", fmt.Errorf("claude-code: %s is a batch shim; install the native claude executable or point %s at it", bin, envClaudeCodeBin)
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
