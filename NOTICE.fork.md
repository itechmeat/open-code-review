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

Each OCR tool loop runs as one persisted Claude Code session: the first round
starts it with `--session-id`, later rounds `--resume` it and send only the new
messages instead of the whole history. If OCR rewrites earlier turns (memory
compression, a new review round) a fresh session starts. On a 24-file review
with haiku this cut uncached input from 1.37M to 0.27M tokens and wall time
from 26 to 9 minutes, with the same findings. The sessions' transcripts are
deleted from `~/.claude/projects` when the review ends.

Smaller fixes and additions that help any provider, each a separate commit so
it can be offered upstream:

| Change | What it gives |
|--------|---------------|
| `ocr llm test --provider <name> [--model <m>]` | test a configured provider that is not the default; for `claude-code` it also prints which `claude` executable answered |
| `ocr config get [key]` | read-only view of the config (`ocr config get provider`); secrets print as `(set)` |
| `ocr review --path <dirs,files,globs>` (also `delegate`) | limit a review to part of the repo; files outside show as `out_of_scope` |
| comma-separated flags keep `{a,b}` globs intact | `--exclude 'packages/{a,b}/**'` works |
| `ocr rules check` shows `Review: selected / excluded (reason)` | no more rules for files that review silently skips, with a hint to `include` them |
| changed files left out of the review are still named in the prompt, marked `[not under review]` | the model knows a matching test or data file changed and can read its diff |
| json/sarif runs print `[ocr] Summary: status=… files=… comments=… tool_calls=… tokens=… provider=… model=… elapsed=… session=… dedup="…"` to stderr | a zero-finding run is distinguishable from a skipped or shallow one; `elapsed` includes dedup, which `manifest.elapsed_ms` (frozen when the review itself ended) does not |
| successful tool calls record their real `duration_ms` in the session | timing in `ocr session` data and the viewer is truthful |
| review merges duplicate findings across file groups with scan's DEDUP_TASK (`--no-dedup` to skip); the session keeps the raw per-file comments | the same problem reported from two files, or twice, comes back once |
| each review checklist in the prompt names its source (OCR built-in, `--rule`, project or global rule file) | findings no longer present OCR's generic defaults as the repository's policy |
| `ocr review -p` hints how to `include` files excluded as `default_path` / `unsupported_ext` | Markdown, tests and similar files are one rule entry away |
| built-in rule for test files (`*.test.*`, `*.spec.*`, `__tests__/`, `*_test.go`, `test_*.py`, `*_test.py`) | once tests are included, they are reviewed for real assertions, consistent fixtures and isolation instead of the generic language checklist |
| `examples/rules/docs.rule.json` | opt-in: includes Markdown/MDX/RST/AsciiDoc and reviews them for accuracy against the code, contradictions and broken examples |
| the claude-code provider skips terminal-integration shims on `PATH` (cmux) and drops `CMUX_SURFACE_ID` | headless runs are not given the terminal's hooks, session ids or MCP server |
| `examples/rules/structured-data.rule.json` | opt-in rule that reviews JSON/YAML values (types, enums, defaults, references), not only key spelling; use with `--rule` or copy into `.opencodereview/rule.json` |

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
  `claudecode_response.go`, `claudecode_session.go`, `claudecode_proc_unix.go`,
  `claudecode_proc_windows.go` and their tests
- `plugins/open-code-review/claude-code/agents/ocr-reviewer.md`
- `plugins/open-code-review/claude-code/commands/review-subscription.md`
- `internal/agent/path_selection.go`, `internal/agent/context_only.go`,
  `internal/config/rules/scope.go`, `internal/model/scope.go`,
  `internal/session/tool_duration.go`, `internal/scan/dedup_comments.go`,
  `internal/config/rules/provenance.go`, and in `cmd/opencodereview/`:
  `config_get_cmd.go`, `path_scope.go`, `run_summary.go`, `review_dedup.go`,
  `llm_close.go`,
  with their tests
- `examples/rules/structured-data.rule.json`, `examples/rules/docs.rule.json`
- `internal/config/rules/rule_docs/tests.md`
- `scripts/fork-install.sh`, `scripts/fork-sync.sh`, `NOTICE.fork.md`
- `docs/superpowers/specs/2026-09-23-claude-code-provider-design.md`,
  `docs/superpowers/plans/2026-09-23-claude-code-provider.md`

Modified (small, local edits, each marked under the license header with
`Modified by Sergey Eroshenkov, 2026; see NOTICE.fork.md.`):

- `internal/llm/protocol.go`, `client.go`, `providers.go`, `providers_test.go`
- `internal/llmloop/loop.go`, `internal/agent/agent.go`, `internal/agent/selection.go`
- `internal/config/rules/system_rules.go`, `system_rules.json` (test-file
  patterns ahead of the language rules)
- `cmd/opencodereview/`: `llm_cmd.go`, `rules_cmd.go`, `review_cmd.go`,
  `delegate_cmd.go`, `scan_cmd.go`, `shared.go`, `shared_flags.go`, `output.go`
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
