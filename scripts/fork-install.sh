#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 alibaba/open-code-review Contributors

# Build ocr from this checkout and install it (macOS, Linux).
#
# Usage: scripts/fork-install.sh [--link-agent]
#   OCR_INSTALL_DIR  target directory for the binary (default: ~/.local/bin)
#   --link-agent     also symlink the ocr-reviewer subagent into ~/.claude/agents;
#                    not needed when the Claude Code plugin is installed, which
#                    ships the same agent
set -euo pipefail

link_agent=0
for arg in "$@"; do
	case "$arg" in
	--link-agent) link_agent=1 ;;
	*)
		echo "unknown option: $arg" >&2
		exit 2
		;;
	esac
done

if ! command -v go >/dev/null 2>&1; then
	echo "go 1.25 or newer is required: https://go.dev/dl/" >&2
	exit 1
fi
if ! command -v claude >/dev/null 2>&1; then
	echo "warning: claude CLI not found on PATH; the claude-code provider needs Claude Code" >&2
	echo "         (other providers work without it)" >&2
fi

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"

bin_dir="${OCR_INSTALL_DIR:-${HOME}/.local/bin}"
mkdir -p "$bin_dir"

version=$(git describe --tags --always 2>/dev/null || echo dev)
commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
built=$(date -u +%Y-%m-%dT%H:%M:%SZ)
# Build beside the target and rename: go build refuses to overwrite a file
# that is not a Go binary, such as a shell wrapper left by an npm install.
go build -ldflags "-X main.Version=${version} -X main.GitCommit=${commit} -X main.BuildDate=${built}" \
	-o "${bin_dir}/ocr.new" ./cmd/opencodereview
mv -f "${bin_dir}/ocr.new" "${bin_dir}/ocr"

if [ "$link_agent" = 1 ]; then
	agent="${repo}/plugins/open-code-review/claude-code/agents/ocr-reviewer.md"
	if [ ! -f "$agent" ]; then
		echo "missing $agent" >&2
		exit 1
	fi
	mkdir -p "${HOME}/.claude/agents"
	ln -sf "$agent" "${HOME}/.claude/agents/ocr-reviewer.md"
fi

resolved=$(command -v ocr || true)
if [ "$resolved" != "${bin_dir}/ocr" ]; then
	echo "warning: 'ocr' resolves to ${resolved:-nothing}, not ${bin_dir}/ocr." >&2
	echo "         Put ${bin_dir} earlier in PATH, or remove the other copy (the npm package" >&2
	echo "         @alibaba-group/open-code-review does not include the claude-code provider)." >&2
fi
"${bin_dir}/ocr" version
