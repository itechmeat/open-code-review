# About this fork

This repository is a fork of
[alibaba/open-code-review](https://github.com/alibaba/open-code-review),
licensed under the Apache License 2.0 like the original. It is maintained by
Sergey Eroshenkov and is not affiliated with or endorsed by Alibaba. The
upstream copyright notices and license are kept unchanged; files this fork
modified carry a notice under their license header, as Apache-2.0 §4(b)
requires.

## What the fork adds

A built-in `claude-code` LLM provider. Every request OCR makes to a model —
grouping, planning, review rounds, the false-positive filter, comment
re-location, memory compression — is served by one headless run of the
official Claude Code CLI (`claude -p`). Reviews then run on whatever `claude`
is logged in with, typically a Claude subscription, with no API key.

OCR's own pipeline is untouched, so sessions, `ocr session …`, `ocr viewer`,
`--resume` and every output format work as upstream documents them.

The fork talks to Claude only through the official `claude` binary. It never
reads, copies or forwards Claude credentials, and it removes
`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`,
`CLAUDE_CODE_USE_BEDROCK` and `CLAUDE_CODE_USE_VERTEX` from the child
environment so a review cannot silently bill an API account or go through
another gateway.

## Usage

```bash
ocr config set providers.claude-code.model sonnet   # once; your default provider stays as it is
ocr review --provider claude-code --audience agent  # workspace changes
ocr review --provider claude-code -c HEAD           # one commit
```

| Variable | Effect |
|----------|--------|
| `OCR_CLAUDE_CODE_EFFORT` | Claude Code reasoning effort: `low` (default), `medium`, `high`, `xhigh`, `max`, or `auto` to let Claude Code decide |
| `OCR_CLAUDE_CODE_BIN` | Path to the `claude` executable when it is not on `PATH` |

Models: `sonnet` (recommended), `opus`, `haiku`, `fable`, or a full model ID.
With many files, `--concurrency 4` keeps a review within subscription rate
limits.

For Claude Code users the plugin also ships an `ocr-reviewer` subagent
(`plugins/open-code-review/claude-code/agents/`) and a
`/open-code-review:review-subscription` command that delegates to it.

## Files

Added:

- `internal/llm/claudecode_client.go`, `claudecode_request.go`,
  `claudecode_response.go`, `claudecode_proc_unix.go`,
  `claudecode_proc_windows.go` and their tests
- `plugins/open-code-review/claude-code/agents/ocr-reviewer.md`
- `plugins/open-code-review/claude-code/commands/review-subscription.md`
- `scripts/fork-sync.sh`, `NOTICE.fork.md`
- `docs/superpowers/specs/2026-09-23-claude-code-provider-design.md`,
  `docs/superpowers/plans/2026-09-23-claude-code-provider.md`

Modified (a few registration lines each, marked under the license header):

- `internal/llm/protocol.go`, `internal/llm/client.go`,
  `internal/llm/providers.go`, `internal/llm/providers_test.go`
- `README.md` and `docs/i18n/README.*.md` (the fork notice at the top)

## Keeping up with upstream

```bash
scripts/fork-sync.sh            # fetch upstream, rebase, test, build, install ocr
scripts/fork-sync.sh --no-test  # same without make check/test
```

The script fast-forwards `main` to `upstream/main`, rebases the
`claude-code-provider` branch onto it, runs `make check test`, installs the
binary as `~/.local/bin/ocr`, and links the `ocr-reviewer` subagent into
`~/.claude/agents/`. Conflicts can only arise in the few modified files
listed above.

## AI assistance

This change was written with AI assistance (Claude Code with Claude Opus),
and reviewed by the maintainer, in line with the upstream contribution
guidelines in `AGENTS.md`.
