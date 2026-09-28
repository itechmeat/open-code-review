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

func TestLLMTestProviderFlagSelectsNonDefaultProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake claude is a shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "claude")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '{\"type\":\"result\",\"is_error\":false,\"result\":\"pong\"}'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCR_CLAUDE_CODE_BIN", fake)

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
