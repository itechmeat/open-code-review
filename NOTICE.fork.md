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
reads, copies or forwards Claude credentials. Before starting `claude` it
removes the variables that would bill an API account, route through another
gateway or remap the requested model (`ANTHROPIC_API_KEY`,
`ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_CUSTOM_HEADERS`,
`ANTHROPIC_MODEL`, `ANTHROPIC_DEFAULT_*_MODEL`, `ANTHROPIC_SMALL_FAST_MODEL`,
`CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`, `CLAUDE_CODE_USE_FOUNDRY`),
and the parent Claude Code session's identity (`CLAUDECODE`,
`CLAUDE_CODE_SESSION_ID`, `CLAUDE_CODE_CHILD_SESSION`,
`CLAUDE_CODE_MESSAGING_SOCKET`, `CLAUDE_CODE_MESSAGING_TOKEN`), so each run is an independent session.

Each request is bounded by the provider's `timeout_sec`, or 15 minutes when
none is set. On Windows the native `claude.exe` is required; npm's
`claude.cmd` shim is refused because `cmd.exe` mangles the arguments.

## Install

The npm package `@alibaba-group/open-code-review` is upstream's build and does
not include this provider, and `go install` cannot fetch the fork (its module
path stays `github.com/alibaba/open-code-review` to keep rebasing trivial). Build
from source instead.

Requirements: Go 1.25 or newer, and for the `claude-code` provider the Claude
Code CLI, logged in (`claude`, then `/login`). The provider relies on
`--json-schema`, `--effort` and `--system-prompt-file`; it was tested with
Claude Code 2.1.280, and an older CLI is reported as "too old for this
provider". All other providers work exactly as upstream documents them.

macOS and Linux:

```bash
git clone -b claude-code-provider https://github.com/itechmeat/open-code-review.git
cd open-code-review
scripts/fork-install.sh                          # installs ~/.local/bin/ocr
OCR_INSTALL_DIR=/usr/local/bin scripts/fork-install.sh   # or elsewhere
```

If an npm-installed `ocr` comes first in `PATH`, the script says so; remove it
or reorder `PATH`. Update later with `git pull && scripts/fork-install.sh`.

Windows: install the native Claude Code (`claude.exe`; npm's `claude.cmd` shim
is refused), then build with `go build -o ocr.exe ./cmd/opencodereview` and put
`ocr.exe` on `PATH`.

Claude Code plugin (review commands plus the `ocr-reviewer` subagent):

```text
/plugin marketplace add itechmeat/open-code-review
/plugin install open-code-review@open-code-review-itechmeat
```

The marketplace has its own name, so it can sit next to upstream's
`open-code-review` marketplace. Without the plugin, `scripts/fork-install.sh
--link-agent` links just the subagent into `~/.claude/agents/`.

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

The plugin's `/open-code-review:review-subscription` command delegates to the
`ocr-reviewer` subagent; upstream's `/open-code-review:review` keeps using
whatever provider is configured.

## Files

Added:

- `internal/llm/claudecode_client.go`, `claudecode_request.go`,
  `claudecode_response.go`, `claudecode_proc_unix.go`,
  `claudecode_proc_windows.go` and their tests
- `plugins/open-code-review/claude-code/agents/ocr-reviewer.md`
- `plugins/open-code-review/claude-code/commands/review-subscription.md`
- `scripts/fork-install.sh`, `scripts/fork-sync.sh`, `NOTICE.fork.md`
- `docs/superpowers/specs/2026-09-23-claude-code-provider-design.md`,
  `docs/superpowers/plans/2026-09-23-claude-code-provider.md`

Modified (a few registration lines each, marked under the license header):

- `internal/llm/protocol.go`, `internal/llm/client.go`,
  `internal/llm/providers.go`, `internal/llm/providers_test.go`
- `.claude-plugin/marketplace.json` (marketplace name and owner, so the fork's
  catalog neither collides with upstream's nor appears to be published by
  Alibaba; JSON has no comments, hence this note instead of a header)
- `README.md` and `docs/i18n/README.*.md` (the fork notice at the top)

## Keeping up with upstream (maintainer)

```bash
scripts/fork-sync.sh            # fetch upstream, rebase, test, then fork-install.sh --link-agent
scripts/fork-sync.sh --no-test  # same without make check/test
```

The script fast-forwards `main` to `upstream/main`, rebases the
`claude-code-provider` branch onto it, runs `make check test`, and reinstalls
through `fork-install.sh`. It never pushes; after a rebase the branch goes up
with `git push --force-with-lease`. Conflicts can only arise in the few
modified files listed above.

## AI assistance

This change was written with AI assistance (Claude Code with Claude Opus),
and reviewed by the maintainer, in line with the upstream contribution
guidelines in `AGENTS.md`.
