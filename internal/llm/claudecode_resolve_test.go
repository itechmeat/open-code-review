// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveClaudeCodePresetNeedsNoURLOrKey(t *testing.T) {
	clearAllEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := `{"provider":"claude-code","providers":{"claude-code":{"model":"sonnet"}}}`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	ep, err := ResolveEndpointWithOptions(path, ResolveOptions{Provider: "claude-code"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if ep.Protocol != ProtocolClaudeCode || ep.Model != "sonnet" || !ep.AmbientAuth {
		t.Fatalf("endpoint = %+v", ep)
	}
	if _, ok := NewLLMClient(ep, nil, nil).(*ClaudeCodeClient); !ok {
		t.Fatal("factory did not return *ClaudeCodeClient")
	}
}
