#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 alibaba/open-code-review Contributors

# Sync this fork with alibaba/open-code-review and reinstall ocr.
#
# Usage: scripts/fork-sync.sh [--no-test] [--no-install]
set -euo pipefail

UPSTREAM_URL="https://github.com/alibaba/open-code-review.git"
BRANCH="claude-code-provider"
BIN_DIR="${HOME}/.local/bin"
AGENTS_DIR="${HOME}/.claude/agents"

run_tests=1
install=1
for arg in "$@"; do
	case "$arg" in
	--no-test) run_tests=0 ;;
	--no-install) install=0 ;;
	*)
		echo "unknown option: $arg" >&2
		exit 2
		;;
	esac
done

repo=$(git rev-parse --show-toplevel)
cd "$repo"

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
	echo "working tree has uncommitted changes; commit or stash them first" >&2
	exit 1
fi

if ! git remote get-url upstream >/dev/null 2>&1; then
	git remote add upstream "$UPSTREAM_URL"
fi
git fetch upstream

git checkout main
git merge --ff-only upstream/main
git checkout "$BRANCH"
git rebase main

if [ "$run_tests" = 1 ]; then
	make check test
fi

if [ "$install" = 1 ]; then
	mkdir -p "$BIN_DIR" "$AGENTS_DIR"
	version=$(git describe --tags --always)
	commit=$(git rev-parse --short HEAD)
	built=$(date -u +%Y-%m-%dT%H:%M:%SZ)
	go build -ldflags "-X main.Version=${version} -X main.GitCommit=${commit} -X main.BuildDate=${built}" \
		-o "${BIN_DIR}/ocr" ./cmd/opencodereview
	ln -sf "${repo}/plugins/open-code-review/claude-code/agents/ocr-reviewer.md" "${AGENTS_DIR}/ocr-reviewer.md"

	resolved=$(command -v ocr || true)
	if [ "$resolved" != "${BIN_DIR}/ocr" ]; then
		echo "warning: 'ocr' resolves to ${resolved:-nothing}, not ${BIN_DIR}/ocr;" >&2
		echo "         remove the npm copy (npm uninstall -g @alibaba-group/open-code-review)" >&2
		echo "         or put ${BIN_DIR} earlier in PATH" >&2
	fi
	"${BIN_DIR}/ocr" version
fi

echo "fork is in sync with upstream/main"
