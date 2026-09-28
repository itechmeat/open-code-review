# Claude Code as the default provider, other providers by parameters: design

Date: 2026-09-28
Status: approved in conversation
Branch: `feat/claude-code-default`
Builds on: `2026-09-23-claude-code-provider-design.md` (the `claude-code` preset)

## Goal

`ocr` works out of the box on a machine where the Claude Code CLI is installed
and logged in: `ocr review` with no config file reviews through
`claude -p` on the user's subscription. Any other provider or model is one
parameter away, per run (`--provider`, `--model`) or permanently
(`ocr config set provider ...`).

Success criteria:

1. On a fresh machine with `claude` on `PATH` and no `~/.opencodereview`,
   `ocr review --from main --to HEAD` completes with the `claude-code`
   provider and the `opus` model, and `ocr llm test` reports that source.
2. `ocr review --provider z-ai-coding --model <id>` uses the configured Z.ai
   entry for that run only, and the next run without flags returns to the
   default.
3. `ocr review --provider claude-code --model sonnet` works without a
   `providers.claude-code` entry in the config file, and without a config
   file at all.
4. `ocr config set provider <name>` still makes that provider the default;
   the config file always wins over the built-in fallback.
5. A subscription usage limit stops the run early with a clear message and a
   resume hint instead of failing every remaining group one by one.
6. The provider is documented everywhere the other presets are: the five
   doc locales, the five README locales, the skill and its plugin mirror.
7. `make check`, `make test`, `make coverage` (90 %) and
   `make english-check` pass; `check-translation-sync.js readmes` passes.

## Non-goals

- No profiles or multiple accounts per preset. A second account of the same
  provider is a `custom_providers` entry, as today.
- No change to the order of the existing endpoint sources (config file,
  `OCR_LLM_*`, `ANTHROPIC_*` env, shell rc). The Claude Code default is
  appended after them, never inserted before.
- No preflight of the Claude login type (subscription versus Console
  account) and no parsing of limit reset times. Both are noted as follow-ups.
- No change to `stripModelSuffix` (`[1m]` handling) and no new TUI screens.

## Design

### 1. Default provider: the last resolver strategy

`ResolveEndpointWithOptions` in `internal/llm/resolver.go` tries the config
file, the `OCR_LLM_*` variables, the `ANTHROPIC_*` variables and the shell rc
exports, then fails with "no valid LLM endpoint configured". A fifth strategy
is appended: if `claudeCodeBinary()` (honouring `OCR_CLAUDE_CODE_BIN`, then
`PATH`) finds a binary, the resolver returns

```
Provider:    claude-code
Protocol:    ProtocolClaudeCode
Model:       opus            (preset DefaultModel, see 2)
AmbientAuth: true
Source:      "claude CLI on PATH" (or "OCR_CLAUDE_CODE_BIN" when that is set)
```

Rules:

- The strategy runs only when the four earlier sources produced nothing. A
  config file that names any provider, even a misconfigured one, keeps its
  current error; the fallback never masks a config mistake.
- When no binary is found, the existing error is extended with one line
  telling the user that installing Claude Code, or configuring a provider,
  would make `ocr` work.
- `ocr llm test` and `ocr config get provider` display the new source so the
  user can see why Claude was chosen.

### 2. Presets usable without a config entry

`Provider` in `internal/llm/providers.go` gains `DefaultModel string`. It is
set to `opus` for `claude-code` and left empty elsewhere, so no other preset
changes behaviour.

`tryProviderConfig` accepts a missing `providers.<name>` entry for a preset
when the preset has `AmbientAuth` or its `EnvVar` is set in the environment.
It then proceeds with a zero entry. The model comes from `--model`, else the
entry, else the top-level config `model`, else `DefaultModel`; if all are
empty the current "no model" error stands (so `bedrock` still needs
`--model`).

The "config file missing" branch of `ResolveEndpointWithOptions`, which today
rejects `--provider`, instead calls `tryProviderConfig` with a synthetic
config that holds only the requested provider. `--provider` therefore works
with or without a file, and switching provider still clears the file's
top-level `model`, as today.

`review`, `scan` and `llm test` need no changes; they already pass
`--provider` and `--model` through `ResolveOptions`.

### 3. `--model` validation

For non-ambient presets, the allow-list for `--model` is the preset list plus
`entry.models`. It is extended with the entry's own `model` and the top-level
config `model`, so a model the user deliberately configured (for example a
newer model the preset list does not know yet) is accepted on the command
line as well. Ambient presets, including `claude-code`, keep accepting any
value, which covers the aliases `opus`, `sonnet`, `haiku` and `fable` and
full model ids.

### 4. Subscription limit ends the run

`claudecode_response.go` already classifies "usage limit" and "hit your
limit" as a limit error. The change: a limit error is marked as fatal for the
run. The review pipeline, on seeing it, cancels the remaining groups, keeps
the results that were already produced, prints one message naming the
provider and the limit, and exits with the existing partial-results code 3
and the `ocr review --resume <id>` hint. Groups already in flight finish or
fail on their own. A missing login and a CLI too old for the provider's
flags fail every later request the same way, so they are fatal for the run
too (added after review).

The broad "rate limit" match stays as a limit error in this version; the
retry-once wrapper still skips it. Narrowing it to distinguish transient API
overload from the subscription window is a follow-up.

`plugins/open-code-review/agents/ocr-reviewer.md` drops the instruction to
create a `providers.claude-code` entry; it is no longer needed.

### 5. Documentation

Every place that documents presets gets the `claude-code` provider, in
English first and then in each locale:

- `pages/src/content/docs/<locale>/configuration.md` (en, zh, ja, ko, ru): a
  "Claude Code (subscription)" section next to Bedrock, covering the default
  behaviour, `--provider` / `--model` without a config entry, the aliases,
  `OCR_CLAUDE_CODE_BIN`, `OCR_CLAUDE_CODE_EFFORT`, the environment scrubbing
  and the limit behaviour.
- `pages/src/content/docs/<locale>/cli-reference.md`: `ocr llm test
  --provider/--model`, the two environment variables, and the resolution
  order ending with the Claude Code fallback.
- `README.md` and the four `docs/i18n/README.*.md`: one short section with
  the same `##` heading position in every locale, because the README sync
  check is blocking.
- `skills/open-code-review/SKILL.md` and its mirror under
  `plugins/open-code-review/skills/`: how an agent picks the provider.
- The 2026-09-23 spec: replace "default stays upstream's" with a pointer to
  this document.

### 6. Tests

- Resolver: fallback chosen when nothing else is configured and a fake
  binary is reachable through `OCR_CLAUDE_CODE_BIN`; not chosen when a config
  file names another provider; error text when no binary exists.
- Presets without an entry: `--provider claude-code` with no file and with a
  file lacking the entry; `--provider bedrock` without `--model` still errors;
  a non-ambient preset without env key still errors.
- `--model`: a model present only in `entry.model` is accepted; unknown model
  for a non-ambient preset still rejected.
- Limit handling: a fake `claude` script that prints a usage-limit error makes
  the run stop after the first failure, publish partial results and exit 3.
- Coverage stays at or above 90 %; `ClaudeCodeBinary` gets direct tests.

## Upstream footprint

Beyond the files the provider already owns (`internal/llm/claudecode_*.go`),
this change touches `resolver.go` (one strategy, the missing-entry rule, the
allow-list), `providers.go` (one field, one preset value), the review
pipeline's error handling for the fatal limit case, and documentation. All of
it is fork-only; nothing is proposed upstream.
