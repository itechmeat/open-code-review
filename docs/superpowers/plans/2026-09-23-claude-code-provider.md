# Claude Code Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve every `ocr review` LLM call through the official `claude -p` CLI on the user's Claude subscription, as a built-in `claude-code` provider.

**Architecture:** A new `ClaudeCodeClient` implements the single-method `llm.LLMClient` interface by spawning one headless `claude -p` per request: system messages become `--system-prompt`, the rest of the conversation is a tagged transcript on stdin, OCR's tools become an `anyOf` JSON schema passed to `--json-schema`, and the structured result maps back to `ChatResponse.ToolCalls`. Registration is three small edits (protocol constant, factory case, preset entry); everything else is new files.

**Tech Stack:** Go 1.25+ (module `github.com/alibaba/open-code-review`), `os/exec`, Claude Code CLI ≥ 2.1.

**Spec:** `docs/superpowers/specs/2026-09-23-claude-code-provider-design.md`

## Global Constraints

- Upstream-file edits limited to: `internal/llm/protocol.go`, `internal/llm/client.go` (one `case`), `internal/llm/providers.go` (one entry), `internal/llm/providers_test.go` (expected list), `internal/llm/protocol_test.go` (tables), `README.md` + `docs/i18n/README.{zh-CN,ja-JP,ko-KR,ru-RU}.md`.
- Each modified Go source file gets, directly under its two SPDX lines: `// Modified by Sergey Eroshenkov, 2026: claude-code provider.`
- New source files carry the SPDX header (`make license-add`).
- English only in source (`make english-check`); comments explain "why" only.
- `make check`, `make test`, `make coverage` (≥ 90%) must pass.
- Child process env drops `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`.
- Binary override env var: `OCR_CLAUDE_CODE_BIN`.
- Commits: English, short, no AI attribution trailers. Nothing is pushed.

## Review Focus

1. Huge diffs (hundreds of KB) — prompt must go through stdin, never argv. Test: 2 MB user message reaches the fake binary intact.
2. Tool arguments the model omits (`arguments` missing or `null`) — must become `"{}"`, not `"null"`, or `code_comment` parsing breaks. Test in Task 3.
3. Cancelled review (Ctrl-C, OCR timeout) — `claude` and its children must die, not linger. Test: context cancel on a sleeping fake returns promptly with `context.Canceled`.
4. CLI prints a JSON error result with exit code 1 (not logged in, usage limit) — user must see an actionable message, not "exit status 1". Test in Task 3.
5. `ANTHROPIC_API_KEY` exported in the user's shell — must not leak to the child and silently bill the API. Test in Task 3.

---

### Task 1: Register protocol and preset

**Files:**
- Modify: `internal/llm/protocol.go`
- Modify: `internal/llm/client.go:452-460` (factory switch)
- Modify: `internal/llm/providers.go` (registry)
- Create: `internal/llm/claudecode_client.go` (stub type only, completed in Task 2/3)
- Test: `internal/llm/protocol_test.go`, `internal/llm/providers_test.go`, `internal/llm/claudecode_resolve_test.go` (new)

**Interfaces:**
- Produces: `const ProtocolClaudeCode = "claude-code"`; `func NewClaudeCodeClient(cfg ClientConfig) *ClaudeCodeClient`; preset `claude-code` (`AmbientAuth: true`).

- [ ] **Step 1: Failing tests** — add `"claude-code"` to the table cases in `protocol_test.go` (normalize `" Claude-Code "` → `claude-code`; validate accepts it), insert `"claude-code"` after `"bedrock"` in `TestListProviders_Order`'s expected slice, and a new resolve test:

```go
func TestResolveClaudeCodePresetNeedsNoURLOrKey(t *testing.T) {
	clearAllEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := `{"provider":"claude-code","providers":{"claude-code":{"model":"sonnet"}}}`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	ep, err := ResolveEndpointWithOptions(path, ResolveOptions{Provider: "claude-code"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if ep.Protocol != ProtocolClaudeCode || ep.Model != "sonnet" || !ep.AmbientAuth {
		t.Fatalf("endpoint = %+v", ep)
	}
	if _, ok := NewLLMClient(ep, nil, nil).(*ClaudeCodeClient); !ok {
		t.Fatal("factory did not return *ClaudeCodeClient")
	}
}
```

- [ ] **Step 2:** `LC_ALL=C go test ./internal/llm/ -run 'Protocol|ListProviders|ClaudeCode'` → FAIL (undefined `ProtocolClaudeCode`).
- [ ] **Step 3: Implement** — constant + `NormalizeProtocol` case + `ValidateProtocol` whitelist and message; factory `case ProtocolClaudeCode: return NewClaudeCodeClient(cfg)`; registry entry:

```go
{
	// Claude Code authenticates with its own login (subscription or
	// whatever `claude` is signed in with), so there is no key or URL here.
	Name:        "claude-code",
	DisplayName: "Claude Code CLI (Claude subscription)",
	Protocol:    ProtocolClaudeCode,
	AmbientAuth: true,
	Models:      []string{"sonnet", "opus", "haiku", "fable", "claude-sonnet-5", "claude-opus-5-5", "claude-haiku-4-5"},
},
```

Stub in `claudecode_client.go`: `type ClaudeCodeClient struct{ bin, model string; timeout time.Duration }`, constructor, and `CompletionsWithCtx` returning `errors.New("not implemented")`.
- [ ] **Step 4:** tests PASS; also fix any resolver test that enumerates AmbientAuth presets.
- [ ] **Step 5:** commit `feat(llm): register claude-code protocol and provider preset`.

### Task 2: Request building (pure)

**Files:**
- Create: `internal/llm/claudecode_request.go`
- Test: `internal/llm/claudecode_request_test.go`

**Interfaces:**
- Produces:
  - `type claudeCodeInvocation struct { Args []string; Stdin string; Structured bool }`
  - `func buildClaudeCodeInvocation(req ChatRequest, defaultModel string) (claudeCodeInvocation, error)`
  - `func claudeCodeSchema(tools []ToolDef, required bool) map[string]any`
  - `func renderClaudeCodeTranscript(msgs []Message) string`

Rules:
- System messages → joined with `\n\n`, plus the fixed bridge instruction (structured mode only) explaining that tools are called by returning `tool_calls`, and results come back as `<message role="tool" tool_call_id="…">`.
- Args: `-p --output-format json --model <m> --tools "" --strict-mcp-config --setting-sources "" --no-session-persistence --system-prompt <s>` and, in structured mode, `--json-schema <compact JSON>`.
- Structured mode iff `len(Tools) > 0 && ToolChoice != "none"`; `ToolChoice == "required"` → `minItems: 1`.
- Each `anyOf` branch: `{"type":"object","description":<tool description>,"properties":{"name":{"const":<name>},"arguments":<parameters or {"type":"object"}>},"required":["name","arguments"]}`.
- Transcript: `<message role="user">…</message>`; assistant text followed by `<tool_call id=".." name="..">{args}</tool_call>` lines; tool messages carry `tool_call_id`. Content via `(*Message).ExtractText()`.
- Empty model after fallback → error `claude-code: no model configured`.

- [ ] **Step 1: Failing tests** — table tests: no tools → no `--json-schema`, `Structured=false`; two tools → schema has 2 branches with `const` names and descriptions; `required` adds `minItems`; `none` disables schema; transcript renders a user/assistant(tool_call)/tool sequence exactly (golden string); system messages never appear in stdin; a 2 MB user message lands in `Stdin` and no arg exceeds 64 KB except system prompt/schema; `req.Model` overrides default.
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** implement.
- [ ] **Step 4:** run → PASS.
- [ ] **Step 5:** commit `feat(llm): build claude -p invocations from chat requests`.

### Task 3: Process execution and response mapping

**Files:**
- Modify: `internal/llm/claudecode_client.go` (replace stub)
- Create: `internal/llm/claudecode_proc_unix.go` (`//go:build !windows`: `Setpgid`, kill `-pid`), `internal/llm/claudecode_proc_windows.go` (`//go:build windows`: `cmd.Process.Kill`)
- Create: `internal/llm/claudecode_response.go`
- Test: `internal/llm/claudecode_client_test.go` (with a `TestMain` fake-binary switch in a new `claudecode_main_test.go`)

**Interfaces:**
- Consumes: `buildClaudeCodeInvocation` (Task 2).
- Produces: `func (c *ClaudeCodeClient) CompletionsWithCtx(ctx context.Context, req ChatRequest) (*ChatResponse, error)`; `var ErrClaudeCodeNotLoggedIn, ErrClaudeCodeUsageLimit error`; `func parseClaudeCodeResult(stdout []byte, structured bool, model string) (*ChatResponse, error)`; `func claudeCodeEnv(environ []string) []string`.

Behavior:
- Binary: `OCR_CLAUDE_CODE_BIN` else `exec.LookPath("claude")`; missing → error naming both.
- Runs in `os.MkdirTemp("", "ocr-claude-*")`, removed afterwards; stdin = invocation stdin; `cmd.Cancel` kills the process group; `cmd.WaitDelay = 5s`; `cfg.Timeout > 0` wraps ctx.
- Result JSON: `{"type":"result","is_error":bool,"result":string,"structured_output":{...},"usage":{input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens}}`.
- Mapping: structured → `ToolCalls[i] = {ID:"call_<i>", Type:"function", Function{Name, Arguments}}` where nil/absent arguments → `"{}"`; `content` → `Content`; `FinishReason` `"tool_calls"`/`"stop"`; usage → `PromptTokens = input+cache_creation+cache_read`, `CompletionTokens = output`, `CacheReadTokens`, `CacheWriteTokens`, `TotalTokens`.
- Errors: `is_error` or exit≠0 → message from `result` (else stderr tail ≤ 2 KB); text containing `not logged in`/`/login`/`invalid api key` (case-insensitive) wraps `ErrClaudeCodeNotLoggedIn`; `usage limit`/`hit your limit`/`rate limit` wraps `ErrClaudeCodeUsageLimit`; structured mode without `structured_output` → error; ctx error takes precedence.
- Not wired into the HTTP retry report (no HTTP attempts exist to finalize).

- [ ] **Step 1: Fake binary harness** — `claudecode_main_test.go`:

```go
func TestMain(m *testing.M) {
	if mode := os.Getenv("OCR_FAKE_CLAUDE"); mode != "" {
		os.Exit(runFakeClaude(mode))
	}
	os.Exit(m.Run())
}
```

`runFakeClaude` writes argv+stdin+env to `$OCR_FAKE_CLAUDE_DUMP` and prints a canned result per mode: `tools`, `text`, `null-args`, `error-login` (exit 1), `error-limit` (exit 1), `garbage`, `sleep`, `missing-structured`. Tests set `OCR_CLAUDE_CODE_BIN=os.Args[0]`.
- [ ] **Step 2: Failing tests** — one per mode plus: env scrub (set `ANTHROPIC_API_KEY` in test env, dump shows it absent, `PATH` present); cancel on `sleep` returns within 3 s with `errors.Is(err, context.Canceled)`; missing binary error mentions `OCR_CLAUDE_CODE_BIN`; `claudeCodeEnv` unit test.
- [ ] **Step 3:** run → FAIL.
- [ ] **Step 4:** implement.
- [ ] **Step 5:** `LC_ALL=C go test ./internal/llm/` → PASS; commit `feat(llm): run requests through claude -p and map results`.

### Task 4: Live verification and measurement

**Files:** none committed except notes appended to the spec's Phase 2 section if numbers change the decision.

- [ ] `make build`; `ocr config set providers.claude-code.model sonnet` (adds the entry; the default provider is left unchanged).
- [ ] `./dist/ocr llm test --provider claude-code` (or the command's actual flag) → reply received.
- [ ] `./dist/ocr review -c HEAD~1 --provider claude-code --model haiku --audience agent` on this repo → completes; `ocr session show <id>` shows manifest `complete`; viewer lists conversations.
- [ ] Record wall time, number of `claude` calls, tokens. Decide Phase 2 (`--resume`) per spec; implement only if replay cost dominates.

### Task 5: Claude Code integration files

**Files:**
- Create: `plugins/open-code-review/claude-code/agents/ocr-reviewer.md`
- Create: `plugins/open-code-review/claude-code/commands/review-subscription.md`

- [ ] Agent frontmatter: `name: ocr-reviewer`, description (when to use), `tools: Bash, Read, Grep, Glob`, `model: haiku` (it only drives OCR and triages). Body: run `ocr review --provider claude-code --audience agent [args]` with 10-minute timeout; if `claude-code` provider not configured, run `ocr config set providers.claude-code.model sonnet`; triage High/Medium/Low as in `commands/review.md`; verify each High finding against the code before reporting; return a compact list `path:line — severity — issue — fix`, plus session ID.
- [ ] Command: dispatches the `ocr-reviewer` subagent with user args.
- [ ] Commit `feat(plugin): add Claude Code subagent for subscription-backed reviews`.

### Task 6: Fork documentation and sync script

**Files:**
- Create: `NOTICE.fork.md`, `scripts/fork-sync.sh`
- Modify: `README.md`, `docs/i18n/README.{zh-CN,ja-JP,ko-KR,ru-RU}.md`

- [ ] `NOTICE.fork.md`: fork of alibaba/open-code-review (Apache-2.0), maintainer Sergey Eroshenkov, list of added/modified files, AI-assistance disclosure (Claude Code, Claude Opus), statement that access to Claude goes only through the official CLI.
- [ ] README (all five): short "About this fork" block at the top + "Claude Code provider (Claude subscription)" usage section.
- [ ] `scripts/fork-sync.sh` (`set -euo pipefail`): ensure `upstream` remote; `git fetch upstream`; `git -C . checkout main && git merge --ff-only upstream/main`; `git checkout claude-code-provider && git rebase main`; `make check test`; `go build -ldflags ... -o ~/.local/bin/ocr ./cmd/opencodereview`; warn if `command -v ocr` is not `~/.local/bin/ocr`; symlink the agent into `~/.claude/agents/`. Flags: `--no-test`, `--no-install`.
- [ ] `make license-add`; commit `docs: describe the fork and add upstream sync script`.

### Task 7: Final gates

- [ ] `make check && make test && make coverage && make english-check`.
- [ ] `git add --renormalize .` check for CRLF.
- [ ] `ocr review --provider claude-code --audience agent --from main --to claude-code-provider --background "…"` (AGENTS.md pre-commit review, now self-hosted); address valid findings.
- [ ] Whole-branch review by a fresh reviewer subagent; fix findings.
- [ ] Install: `npm uninstall -g @alibaba-group/open-code-review` (reversible), run `scripts/fork-sync.sh --no-test` install part; verify `command -v ocr` and `ocr version`.
- [ ] Hand off: GitHub fork creation + remotes + push wait for explicit user approval.
