// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installFakeClaude points OCR_CLAUDE_CODE_BIN at a script that answers every
// request with "pong".
func installFakeClaude(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake claude is a shell script")
	}
	fake := filepath.Join(dir, "claude")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '{\"type\":\"result\",\"is_error\":false,\"result\":\"pong\"}'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCR_CLAUDE_CODE_BIN", fake)
	return fake
}

// clearEndpointEnv removes every endpoint source a developer machine may
// export, so only the config path and the fake CLI decide.
func clearEndpointEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"OCR_LLM_URL", "OCR_LLM_TOKEN", "OCR_LLM_MODEL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_MODEL"} {
		t.Setenv(k, "")
	}
	setTestHome(t, t.TempDir())
}

func TestLLMTestFallsBackToClaudeCodeWithoutConfig(t *testing.T) {
	clearEndpointEnv(t)
	dir := t.TempDir()
	fake := installFakeClaude(t, dir)
	cfg := filepath.Join(dir, "absent.json")

	for _, tc := range []struct {
		name, provider, model string
		want                  []string
	}{
		{"built-in default", "", "", []string{"Source: OCR_CLAUDE_CODE_BIN", "Model:  opus"}},
		{"explicit provider", "claude-code", "haiku", []string{"Source: provider:claude-code", "Model:  haiku"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() { llmTestOpts = llmTestOptions{} })
			llmTestOpts = llmTestOptions{provider: tc.provider, model: tc.model}
			var runErr error
			out := captureStdout(t, func() { runErr = runLLMTestWithConfigPath(cfg) })
			if runErr != nil {
				t.Fatalf("llm test: %v", runErr)
			}
			for _, want := range append(tc.want, "CLI:    "+fake, "pong") {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestLLMTestProviderFlagSelectsNonDefaultProvider(t *testing.T) {
	dir := t.TempDir()
	fake := installFakeClaude(t, dir)

	cfg := filepath.Join(dir, "config.json")
	// The default provider is unusable here: only the flag can make the test pass.
	body := `{"provider":"anthropic","providers":{"anthropic":{"model":"x"},"claude-code":{"model":"sonnet"}}}`
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { llmTestOpts = llmTestOptions{} })
	if err := llmTestCmd.Flags().Set("provider", "claude-code"); err != nil {
		t.Fatal(err)
	}
	if err := llmTestCmd.Flags().Set("model", "haiku"); err != nil {
		t.Fatal(err)
	}

	var runErr error
	out := captureStdout(t, func() { runErr = runLLMTestWithConfigPath(cfg) })
	if runErr != nil {
		t.Fatalf("llm test: %v", runErr)
	}
	for _, want := range []string{"CLI:    " + fake, "Model:  haiku", "pong", "Connection test successful"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
