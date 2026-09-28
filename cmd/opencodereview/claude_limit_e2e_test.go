// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/session"
)

// fakeClaudeLimitScript answers the grouping call with one group per file,
// completes the review of the first file it sees and fails every other file
// with $FAKE_CLAUDE_FAILURE, a subscription usage limit by default. Each
// non-grouping call appends the file it was about to its log.
const fakeClaudeLimitScript = `#!/bin/sh
state="$(dirname "$0")"
input="$(cat)"
system=""
prev=""
for a in "$@"; do
	if [ "$prev" = "--system-prompt-file" ]; then system="$(cat "$a")"; fi
	prev="$a"
done
case "$system" in
*"file grouping assistant"*)
	printf '%s' '{"type":"result","is_error":false,"result":"[{\"label\":\"g1\",\"files\":[0]},{\"label\":\"g2\",\"files\":[1]},{\"label\":\"g3\",\"files\":[2]},{\"label\":\"g4\",\"files\":[3]}]"}'
	exit 0
	;;
esac
file=unknown
for m in ALPHA BETA GAMMA DELTA; do
	case "$input" in *"MARKER_$m"*) file="$m" ;; esac
done
echo "$file" >>"$state/calls.log"
[ -f "$state/first" ] || echo "$file" >"$state/first"
if [ "$(cat "$state/first")" = "$file" ]; then
	printf '%s' '{"type":"result","is_error":false,"result":"","structured_output":{"content":"","tool_calls":[{"name":"task_done","arguments":{"state":"DONE"}}]},"usage":{"input_tokens":1,"output_tokens":1}}'
	exit 0
fi
printf '{"type":"result","is_error":true,"result":"%s"}' "${FAKE_CLAUDE_FAILURE:-Claude AI usage limit reached|1760000000}"
exit 1
`

func TestReviewE2E_ClaudeCodeAccountErrorsStopTheRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake claude is a shell script")
	}
	for _, tc := range []struct{ name, failure, want string }{
		{"usage limit", "", "Claude usage limit reached"},
		{"not logged in", "Not logged in · Please run /login", "not logged in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FAKE_CLAUDE_FAILURE", tc.failure)
			testClaudeCodeFailureStopsTheRun(t, tc.want)
		})
	}
}

func testClaudeCodeFailureStopsTheRun(t *testing.T, want string) {
	repoDir := retryTestRepo(t)
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, k := range []string{"OCR_LLM_URL", "OCR_LLM_TOKEN", "OCR_LLM_MODEL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_MODEL"} {
		t.Setenv(k, "")
	}
	state := t.TempDir()
	fake := filepath.Join(state, "claude")
	if err := os.WriteFile(fake, []byte(fakeClaudeLimitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCR_CLAUDE_CODE_BIN", fake)

	var err error
	var out string
	errOut := captureStderr(t, func() {
		out = captureStdout(t, func() {
			err = runReview([]string{"--repo", repoDir, "--from", "HEAD~1", "--to", "HEAD", "--format", "json", "--concurrency", "1"})
		})
	})

	if err == nil || exitCodeFor(err) != exitPartial {
		t.Fatalf("a limit after one reviewed file must exit %d: %v\nstderr: %s", exitPartial, err, errOut)
	}
	for _, w := range []string{"Run stopped early: claude-code cannot serve further requests", want, "--resume"} {
		if !strings.Contains(errOut, w) {
			t.Errorf("stderr lacks %q:\n%s", w, errOut)
		}
	}

	data, readErr := os.ReadFile(filepath.Join(state, "calls.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	calls := strings.Fields(string(data))
	limited := 0
	first, _ := os.ReadFile(filepath.Join(state, "first"))
	for _, c := range calls {
		if c != strings.TrimSpace(string(first)) {
			limited++
		}
	}
	if limited != 1 {
		t.Fatalf("the run must stop after the first account error, got %d failed call(s): %v", limited, calls)
	}

	var got jsonOutput
	if e := json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", e, out)
	}
	m := got.Manifest
	if m == nil || m.TerminalState != session.StatePartial {
		t.Fatalf("manifest = %+v, want a partial run", m)
	}
	if len(m.Coverage.Completed) != 1 || len(m.Coverage.Failed) != 3 {
		t.Fatalf("coverage completed=%d failed=%d, want 1 and 3", len(m.Coverage.Completed), len(m.Coverage.Failed))
	}
	for _, item := range m.Coverage.Failed {
		if item.Classification != session.FailureProvider {
			t.Errorf("item %s class = %q, want provider", item.ItemID, item.Classification)
		}
	}
}
