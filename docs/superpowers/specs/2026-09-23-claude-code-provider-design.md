# Claude Code provider for OCR — design

Date: 2026-09-23
Status: approved in conversation, pending written-spec review
Branch: `claude-code-provider` (fork of `alibaba/open-code-review`)

## Goal

Run the full `ocr review` pipeline with every LLM call served by the local,
official Claude Code CLI (`claude -p`) under the user's Claude subscription, so
that a Claude Code agent (or its subagent) can launch OCR and get OCR's
precision-oriented review without an API key.

Success criteria:

1. `ocr review --provider claude-code --audience agent` completes on a real
   diff with no API key, no base URL and no config file edits.
2. Every OCR stage works unchanged: grouping, plan, review rounds, review
   filter, re-location, memory compression, sessions, `ocr session *`,
   `ocr viewer`, `--resume`, JSON/SARIF/agent output.
3. Rebasing the fork on a new upstream release touches only the handful of
   registration lines listed under "Upstream footprint".
4. `make check`, `make test`, `make coverage` (90%) and `make english-check`
   pass.

## Non-goals

- No reuse of subscription OAuth tokens outside the `claude` binary, no
  proxying of Anthropic endpoints. Claude Code is the only thing that talks to
  Anthropic.
- No replacement of OCR's orchestration (the earlier "delegate v2" idea is
  dropped: it would duplicate ~2k lines of pipeline and session-manifest logic
  that upstream changes often).
- No custom-provider TUI support for the new protocol in the first version; it
  ships as a built-in preset only.
- No mapping of `temperature` / `max_tokens`: `claude -p` exposes neither.

## Architecture

```
ocr review ──► agent / llmloop (unchanged)
                 │  llm.LLMClient.CompletionsWithCtx(ChatRequest)
                 ▼
          ClaudeCodeClient  (new, internal/llm/claudecode_client.go)
                 │  one headless process per request
                 ▼
          claude -p  (official CLI, subscription login)
```

`llm.LLMClient` is a single-method interface and is the only seam used. A new
protocol constant `ProtocolClaudeCode = "claude-code"` routes
`NewLLMClient` to `ClaudeCodeClient`. A built-in provider preset
`claude-code` with `AmbientAuth: true` lets the resolver accept it without URL
or API key, through the same generic path Bedrock already uses.

## Request mapping

Each `CompletionsWithCtx` call spawns:

```
claude -p
  --output-format json
  --model <ChatRequest.Model>
  --system-prompt <joined system messages>
  --tools ""                 # no built-in Claude Code tools
  --strict-mcp-config        # no MCP servers
  --setting-sources ""       # no user/project settings, hooks, plugins
  --no-session-persistence   # nothing written to ~/.claude/projects
  [--json-schema <schema>]   # only when tools are offered
```

- **Prompt**: all non-system messages are rendered into one transcript on
  **stdin** (argv would hit `ARG_MAX` on large diffs, and an empty stdin makes
  the CLI wait 3 s). Format, per message:
  `<message role="user|assistant|tool" [tool_call_id=".."]>…</message>`;
  assistant tool calls render as
  `<tool_call id=".." name="..">{json args}</tool_call>`. `[]ContentBlock`
  content is flattened to its text parts.
- **Tools → structured output**: when `ChatRequest.Tools` is non-empty and
  `ToolChoice != "none"`, the client builds
  ```json
  {"type":"object","properties":{
     "content":{"type":"string"},
     "tool_calls":{"type":"array","items":{"anyOf":[
        {"type":"object","properties":{
           "name":{"const":"<tool>"},
           "arguments":<tool.parameters>},
         "required":["name","arguments"]}, …]}}},
   "required":["tool_calls"]}
  ```
  `"minItems":1` is added unless `ToolChoice == "auto"` explicitly: OCR never
  sets tool_choice, but every tool-bearing request it makes expects an action
  (a live filter call returned an empty list instead of `approve_all_comments`).
  Verified on 2026-09-23: `claude -p --json-schema` enforces `anyOf` + `const`
  exactly.
- **Bridge instruction** (prepended to the tool-mode system prompt): the only
  invocable tool is StructuredOutput, OCR's tools are requested inside it,
  `content` is never delivered as a result. Without it a live haiku run wrote
  findings into `content` and only called `task_done`.
- **Effort**: `--effort low` by default, override with
  `OCR_CLAUDE_CODE_EFFORT` (`low|medium|high|xhigh|max`, or `auto` to omit the
  flag). Thinking time dominates latency: on a real main-task request sonnet
  took ~12 s at low, ~27 s at medium, 25–60 s with the flag unset; one-file
  review went from 6.5 min (haiku, unset) to 2 min (sonnet, low).
- **No tools / `ToolChoice == "none"`**: no schema; the answer is the plain
  `result` text (grouping, re-location and compression tasks expect text or
  JSON in content).
- **Working directory**: a per-process temp dir, removed afterwards, so no
  project `CLAUDE.md` / `AGENTS.md` is picked up.
- **Environment**: the child inherits the parent environment except
  `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and `CLAUDE_CODE_USE_BEDROCK` /
  `CLAUDE_CODE_USE_VERTEX`, which would silently switch Claude Code from the
  subscription to paid API billing. Documented in the provider notes.
- **Binary**: `claude` from `PATH`; override with `OCR_CLAUDE_CODE_BIN`.
- **Timeout / cancel**: the request context bounds the process; on cancel the
  whole process group is killed. New `claudecode_proc_unix.go` /
  `claudecode_proc_windows.go` in `internal/llm` set `Setpgid` and kill the
  group (the CLI needs no TTY, unlike `keycmd_unix.go`, which deliberately
  avoids `Setpgid`).

## Response mapping

The CLI's JSON result (`type:"result"`) is mapped to `llm.ChatResponse`:

- `structured_output.tool_calls[i]` → `ToolCall{ID:"call_<n>", Type:"function",
  Function{Name, Arguments: compact JSON}}`; `structured_output.content` →
  `Content`.
- Text mode: `result` → `Content`.
- `FinishReason`: `"tool_calls"` when calls exist, else `"stop"`.
- `usage.input_tokens + cache_creation + cache_read` → `PromptTokens`;
  `output_tokens` → `CompletionTokens`; cache fields → `CacheReadTokens` /
  `CacheWriteTokens`. Session token stats stay truthful.
- `model` → from the CLI result when present, else the requested model.

## Errors

- Non-zero exit, `is_error:true`, or unparsable output → error carrying the
  CLI's message (stderr tail, capped). Messages that indicate usage limits or
  not being logged in are wrapped in distinct, testable error values so OCR's
  warnings and exit code tell the user what to do (`claude /login`, wait for
  the limit window).
- The client does not feed the HTTP retry report: that report is built from
  HTTP attempts, and Claude Code performs its own API retries internally.
  One process per request; OCR's higher-level stage fallbacks still apply.

## Provider preset and UX

```go
{Name: "claude-code", DisplayName: "Claude Code CLI (Claude subscription)",
 Protocol: ProtocolClaudeCode, AmbientAuth: true,
 Models: []string{"sonnet", "opus", "haiku", "fable",
                  "claude-sonnet-5", "claude-opus-5-5", "claude-haiku-4-5"}}
```

Aliases come first so the preset follows Claude Code's own model mapping without
edits to OCR. Usage:

```
ocr review --provider claude-code --model sonnet --audience agent
ocr config set provider claude-code      # make it the default
```

Recommended `--concurrency 4` to stay within subscription rate limits; the
default stays upstream's.

## Claude Code integration

- `plugins/open-code-review/claude-code/agents/ocr-reviewer.md` — subagent
  definition: runs `ocr review --provider claude-code --audience agent
  [args]` via Bash with a long timeout, triages findings the way the existing
  `/review` command does, returns only the vetted list. Installed to
  `~/.claude/agents/` by the sync script (symlink).
- The existing `plugins/open-code-review/claude-code/commands/review.md` is
  left untouched (upstream file); a new `review-subscription.md` command
  delegates to the subagent.

## Upstream footprint (edits to existing files)

| File | Change |
|------|--------|
| `internal/llm/protocol.go` | constant, `NormalizeProtocol` case, `ValidateProtocol` whitelist + message |
| `internal/llm/client.go` | one `case ProtocolClaudeCode:` in `NewLLMClient` |
| `internal/llm/providers.go` | one registry entry |
| `README.md` + 4 `docs/i18n/README.*.md` | fork notice section + provider usage |

Everything else is new files. Each modified source file gets one line under its
SPDX header: `// Modified by Sergey Eroshenkov, 2026: claude-code provider.`
(Apache-2.0 §4(b)). A `NOTICE.fork.md` at the root summarises the fork and its
changes.

## Fork hygiene and sync

- GitHub fork of `alibaba/open-code-review` under the user's account:
  `origin` = fork, `upstream` = alibaba. Work lives on `claude-code-provider`;
  `main` mirrors upstream. Nothing is pushed without explicit approval.
- `scripts/fork-sync.sh`: `git fetch upstream` → fast-forward `main` →
  rebase `claude-code-provider` → `make check test` → `go build` →
  install `ocr` to `~/.local/bin` (ahead of the npm copy in `PATH`) → refresh
  the agent symlink. Stops on the first failure.
- Commits follow AGENTS.md: English, short, no AI attribution trailers. The
  fork README discloses that AI tools were used to write the change (AGENTS.md
  rule 1, relevant if it is ever offered upstream as a PR).
- Nothing is published under alibaba's names; the npm package name is not
  reused.

## Testing

- Unit tests with a fake `claude` binary (Go test helper re-executing the test
  binary via `OCR_CLAUDE_CODE_BIN`) asserting: argv flags, stdin transcript,
  schema shape for 0/1/n tools and each `ToolChoice`, response mapping (tool
  calls, text, usage), error paths (exit code, `is_error`, bad JSON, limit and
  login messages), context cancel kills the process, env scrubbing.
- Protocol/provider tests extend the existing table tests
  (`protocol_test.go`, `providers_test.go`, resolver tests for the preset
  resolving without URL/key).
- Live smoke (manual, not in CI): `ocr review -c HEAD --provider claude-code
  --model haiku` on this repo, then `ocr session show` and `ocr viewer`.

## Phase 2 (only if measurements justify it)

One process per request resends the whole history each tool round. If the
live run shows latency or token usage dominated by replays, keep one Claude
Code session per `ChatRequest.SessionID` via `--resume`, sending only the
messages appended since the previous call, and fall back to a fresh session
when the prefix changed (memory compression). Measured on 2026-09-23:
per-call overhead ≈ 1.2 s, haiku call ≈ 3.6 s, sonnet ≈ 6.5 s.

## Risks

- Claude Code CLI flags may change: all flags live in one builder function with
  a test; `ocr llm test --provider claude-code` surfaces breakage quickly.
- Subscription rate limits under parallel groups: mitigated by
  `--concurrency`; limit errors are reported distinctly.
- Structured-output turns cost an extra internal tool turn inside Claude Code
  (`num_turns: 3` observed); accepted.
