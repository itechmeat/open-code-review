// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClaudeBin creates an executable stand-in for the claude CLI and points
// OCR_CLAUDE_CODE_BIN at it. Resolution only checks that the file exists.
func fakeClaudeBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envClaudeCodeBin, bin)
	return bin
}

func missingConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "absent", "config.json")
}

func writeRawConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeCodePresetDefaultModel(t *testing.T) {
	for _, p := range ListProviders() {
		want := ""
		if p.Name == "claude-code" {
			want = "opus"
		}
		if p.DefaultModel != want {
			t.Errorf("preset %q DefaultModel = %q, want %q", p.Name, p.DefaultModel, want)
		}
	}
}

func TestResolveClaudeCodeProviderWithoutConfigEntry(t *testing.T) {
	clearAllEnv(t)
	fakeClaudeBin(t)

	cases := []struct {
		name, path, model, want string
	}{
		{"no config file", missingConfigPath(t), "", "opus"},
		{"no config file with --model", missingConfigPath(t), "sonnet", "sonnet"},
		{"file without the entry", writeRawConfig(t, `{"provider":"z-ai-coding","model":"glm-5","providers":{"z-ai-coding":{"api_key":"k"}}}`), "", "opus"},
		{"file without the entry, any model id", writeRawConfig(t, `{"provider":"z-ai-coding","providers":{"z-ai-coding":{"api_key":"k"}}}`), "claude-opus-9", "claude-opus-9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep, err := ResolveEndpointWithOptions(tc.path, ResolveOptions{Provider: "claude-code", Model: tc.model})
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if ep.Provider != "claude-code" || ep.Protocol != ProtocolClaudeCode || ep.Model != tc.want || !ep.AmbientAuth {
				t.Fatalf("endpoint = %+v", ep)
			}
		})
	}
}

func TestResolvePresetWithoutEntryStillNeedsModelOrKey(t *testing.T) {
	clearAllEnv(t)

	t.Run("bedrock without --model", func(t *testing.T) {
		_, err := ResolveEndpointWithOptions(missingConfigPath(t), ResolveOptions{Provider: "bedrock"})
		if err == nil || !strings.Contains(err.Error(), "no model configured") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bedrock with --model", func(t *testing.T) {
		ep, err := ResolveEndpointWithOptions(missingConfigPath(t), ResolveOptions{Provider: "bedrock", Model: "anthropic.claude-x"})
		if err != nil || ep.Protocol != ProtocolAnthropicBedrock {
			t.Fatalf("endpoint = %+v, err = %v", ep, err)
		}
	})
	t.Run("non-ambient preset without env key", func(t *testing.T) {
		_, err := ResolveEndpointWithOptions(missingConfigPath(t), ResolveOptions{Provider: "anthropic", Model: "claude-opus-5"})
		if err == nil || !strings.Contains(err.Error(), "not configured in providers section") || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("non-ambient preset with env key", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "sk-test")
		ep, err := ResolveEndpointWithOptions(missingConfigPath(t), ResolveOptions{Provider: "anthropic", Model: "claude-opus-5"})
		if err != nil || ep.Token != "sk-test" || ep.Model != "claude-opus-5" {
			t.Fatalf("endpoint = %+v, err = %v", ep, err)
		}
	})
	t.Run("custom provider without file", func(t *testing.T) {
		_, err := ResolveEndpointWithOptions(missingConfigPath(t), ResolveOptions{Provider: "my-gateway"})
		if err == nil || !strings.Contains(err.Error(), "custom_providers") {
			t.Fatalf("err = %v", err)
		}
	})
}
