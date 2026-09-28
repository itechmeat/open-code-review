// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for the claude CLI: tests point
// OCR_CLAUDE_CODE_BIN at os.Args[0] and select a canned behavior through
// OCR_FAKE_CLAUDE, so the client's real process handling runs on every
// platform without a shell script.
func TestMain(m *testing.M) {
	if mode := os.Getenv("OCR_FAKE_CLAUDE"); mode != "" {
		os.Exit(runFakeClaude(mode))
	}
	os.Exit(m.Run())
}

type fakeClaudeDump struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin"`
	Env   []string `json:"env"`
	Dir   string   `json:"dir"`
	// SystemPrompt is the content of the --system-prompt-file the client wrote.
	SystemPrompt string `json:"system_prompt"`
}

func runFakeClaude(mode string) int {
	stdin, _ := io.ReadAll(os.Stdin)
	if path := os.Getenv("OCR_FAKE_CLAUDE_DUMP"); path != "" {
		dir, _ := os.Getwd()
		var system []byte
		for i, a := range os.Args {
			if a == "--system-prompt-file" && i+1 < len(os.Args) {
				system, _ = os.ReadFile(os.Args[i+1])
			}
		}
		data, _ := json.Marshal(fakeClaudeDump{Args: os.Args[1:], Stdin: string(stdin), Env: os.Environ(), Dir: dir, SystemPrompt: string(system)})
		_ = os.WriteFile(path, data, 0o600)
		// Every call is also appended, for tests that span several requests.
		if f, err := os.OpenFile(path+".all", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(append(data, '\n'))
			_ = f.Close()
		}
	}
	// Like the real CLI, a persisted session leaves a transcript under the
	// config dir's projects tree.
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		for i, a := range os.Args {
			if a == "--session-id" && i+1 < len(os.Args) {
				dir := filepath.Join(cfg, "projects", "-tmp-ocr-claude-x")
				_ = os.MkdirAll(dir, 0o700)
				_ = os.WriteFile(filepath.Join(dir, os.Args[i+1]+".jsonl"), []byte("{}\n"), 0o600)
			}
		}
	}
	if mode == "flaky" {
		marker := os.Getenv("OCR_FAKE_CLAUDE_DUMP") + ".failed-once"
		if _, err := os.Stat(marker); err != nil {
			_ = os.WriteFile(marker, nil, 0o600)
			fmt.Print(`{"type":"result","is_error":true,"result":"API Error: safeguards flagged this message"}`)
			return 1
		}
		mode = "tools"
	}
	if mode == "resume-fails" {
		for _, a := range os.Args {
			if a == "--resume" {
				fmt.Print(`{"type":"result","is_error":true,"result":"No conversation found with session ID"}`)
				return 1
			}
		}
		mode = "tools"
	}
	usage := `"usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":3,"cache_read_input_tokens":2}`
	switch mode {
	case "tools":
		fmt.Printf(`{"type":"result","is_error":false,"result":"","structured_output":{"content":"checked","tool_calls":[{"name":"code_comment","arguments":{"content":"x"}},{"name":"task_done","arguments":{"state":"DONE"}}]},%s}`, usage)
	case "null-args":
		fmt.Printf(`{"type":"result","is_error":false,"structured_output":{"tool_calls":[{"name":"task_done","arguments":null},{"name":"task_done"}]},%s}`, usage)
	case "text":
		fmt.Printf(`{"type":"result","is_error":false,"result":"hello","structured_output":null,%s}`, usage)
	case "missing-structured":
		fmt.Printf(`{"type":"result","subtype":"error_max_structured_output_retries","is_error":false,"result":"I refuse to use the schema",%s}`, usage)
	case "error-login":
		fmt.Print(`{"type":"result","is_error":true,"result":"Not logged in · Please run /login"}`)
		return 1
	case "error-limit":
		fmt.Print(`{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1760000000"}`)
		return 1
	case "old-cli":
		fmt.Fprint(os.Stderr, "error: unknown option '--effort'")
		return 1
	case "stderr-only":
		fmt.Fprint(os.Stderr, "boom: unexpected failure")
		return 2
	case "garbage":
		fmt.Print("not json")
	case "sleep":
		time.Sleep(30 * time.Second)
	default:
		fmt.Fprintf(os.Stderr, "unknown fake mode %q", mode)
		return 3
	}
	return 0
}
