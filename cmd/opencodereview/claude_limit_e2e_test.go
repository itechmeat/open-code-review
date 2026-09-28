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
		{"session limit", "You've hit your session limit · resets 5pm (Europe/Belgrade)", "resets 5pm (Europe/Belgrade)"},
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

func TestScanE2E_ClaudeCodeFatalErrorStopsTheScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake claude is a shell script")
	}
	t.Setenv("FAKE_CLAUDE_FAILURE", "You've hit your session limit · resets 5pm (Europe/Belgrade)")
	repoDir := retryTestRepo(t)
	state := useFakeClaudeScript(t)

	var err error
	var out string
	errOut := captureStderr(t, func() {
		out = captureStdout(t, func() {
			opts, perr := parseScanFlags([]string{"--repo", repoDir, "--format", "json", "--concurrency", "1", "--no-dedup", "--no-summary"})
			if perr != nil {
				t.Fatal(perr)
			}
			err = executeScan(opts)
		})
	})

	// Scan exits 0 on partial coverage, as it does when single files fail.
	if err != nil {
		t.Fatalf("a scan with a scanned file must publish its results: %v\nstderr: %s", err, errOut)
	}
	for _, w := range []string{"Run stopped early: claude-code cannot serve further requests", "resets 5pm (Europe/Belgrade)", "ocr scan --resume"} {
		if !strings.Contains(errOut, w) {
			t.Errorf("stderr lacks %q:\n%s", w, errOut)
		}
	}
	if failed := fakeClaudeFailedCalls(t, state); failed != 1 {
		t.Fatalf("the scan must stop after the first account error, got %d failed call(s)", failed)
	}
	var got jsonOutput
	if e := json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", e, out)
	}
	stopped := false
	for _, w := range got.Warnings {
		stopped = stopped || w.Type == "run_stopped"
	}
	if !stopped {
		t.Errorf("warnings lack run_stopped: %+v", got.Warnings)
	}
}

func TestScanE2E_ClaudeCodeFatalErrorBeforeAnyFileFailsTheScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake claude is a shell script")
	}
	t.Setenv("FAKE_CLAUDE_FAILURE", "Not logged in · Please run /login")
	repoDir := retryTestRepo(t)
	state := useFakeClaudeScript(t)
	// No file ever succeeds: "first" names a file that does not exist.
	if werr := os.WriteFile(filepath.Join(state, "first"), []byte("NONE\n"), 0o600); werr != nil {
		t.Fatal(werr)
	}

	var err error
	errOut := captureStderr(t, func() {
		captureStdout(t, func() {
			opts, perr := parseScanFlags([]string{"--repo", repoDir, "--format", "json", "--concurrency", "1", "--no-plan", "--no-dedup", "--no-summary"})
			if perr != nil {
				t.Fatal(perr)
			}
			err = executeScan(opts)
		})
	})

	if err == nil || exitCodeFor(err) != 1 || !strings.Contains(err.Error(), "before any file was scanned") {
		t.Fatalf("err = %v (exit %d), want a failed scan", err, exitCodeFor(err))
	}
	if !strings.Contains(errOut, "Run stopped early: claude-code cannot serve further requests") || !strings.Contains(errOut, "/login") {
		t.Errorf("stderr lacks the run stop:\n%s", errOut)
	}
	if failed := fakeClaudeFailedCalls(t, state); failed != 1 {
		t.Fatalf("the scan must stop after the first account error, got %d failed call(s)", failed)
	}
}

// useFakeClaudeScript installs fakeClaudeLimitScript as the claude CLI with a
// throwaway HOME and returns the script's state directory.
func useFakeClaudeScript(t *testing.T) string {
	t.Helper()
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
	return state
}

// fakeClaudeFailedCalls counts the calls about a file other than the one the
// script lets succeed.
func fakeClaudeFailedCalls(t *testing.T, state string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(state, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(state, "first"))
	failed := 0
	for _, c := range strings.Fields(string(data)) {
		if c != strings.TrimSpace(string(first)) {
			failed++
		}
	}
	return failed
}
