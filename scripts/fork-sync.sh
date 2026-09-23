#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 alibaba/open-code-review Contributors

# Maintainer tool: sync this fork with alibaba/open-code-review, then reinstall
# ocr via fork-install.sh. Users only need fork-install.sh.
#
# Usage: scripts/fork-sync.sh [--no-test] [--no-install]
set -euo pipefail

UPSTREAM_URL="https://github.com/alibaba/open-code-review.git"
BRANCH="claude-code-provider"

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
if ! git merge --ff-only upstream/main; then
	echo "local main has diverged from upstream/main; main must stay a mirror of upstream" >&2
	echo "inspect with: git log --oneline upstream/main..main" >&2
	exit 1
fi
git checkout "$BRANCH"
if ! git rebase main; then
	echo "rebase stopped on conflicts (expected only in the files listed in NOTICE.fork.md)" >&2
	echo "resolve them, run 'git rebase --continue', then rerun this script; or 'git rebase --abort'" >&2
	exit 1
fi

if [ "$run_tests" = 1 ]; then
	make check test
fi

if [ "$install" = 1 ]; then
	"${repo}/scripts/fork-install.sh" --link-agent
fi

echo "fork is in sync with upstream/main"
