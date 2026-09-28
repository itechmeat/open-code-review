// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveFallsBackToClaudeCodeWhenNothingConfigured(t *testing.T) {
	clearAllEnv(t)
	fakeClaudeBin(t)

	for name, path := range map[string]string{
		"no config file":    missingConfigPath(t),
		"empty config file": writeRawConfig(t, `{}`),
	} {
		t.Run(name, func(t *testing.T) {
			ep, err := ResolveEndpoint(path)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if ep.Provider != "claude-code" || ep.Protocol != ProtocolClaudeCode || ep.Model != "opus" || !ep.AmbientAuth {
				t.Fatalf("endpoint = %+v", ep)
			}
			if ep.Source != envClaudeCodeBin {
				t.Fatalf("source = %q, want %q", ep.Source, envClaudeCodeBin)
			}
		})
	}
}

func TestResolveFallbackFindsClaudeOnPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake claude on PATH needs an executable bit")
	}
	clearAllEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envClaudeCodeBin, "")
	t.Setenv("PATH", dir)

	ep, err := ResolveEndpointWithModelOverride(missingConfigPath(t), "haiku")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if ep.Source != "claude CLI on PATH" || ep.Model != "haiku" || ep.Provider != "claude-code" {
		t.Fatalf("endpoint = %+v", ep)
	}
}

func TestResolveFallbackNeverMasksConfiguredSources(t *testing.T) {
	clearAllEnv(t)
	fakeClaudeBin(t)

	t.Run("misconfigured provider keeps its error", func(t *testing.T) {
		path := writeRawConfig(t, `{"provider":"anthropic","providers":{"anthropic":{"model":"claude-opus-5"}}}`)
		_, err := ResolveEndpoint(path)
		if err == nil || !strings.Contains(err.Error(), "no api_key") {
			t.Fatalf("err = %v, want the provider's own error", err)
		}
	})
	t.Run("configured provider wins", func(t *testing.T) {
		path := writeRawConfig(t, `{"provider":"z-ai-coding","providers":{"z-ai-coding":{"api_key":"k","model":"glm-5"}}}`)
		ep, err := ResolveEndpoint(path)
		if err != nil || ep.Provider != "z-ai-coding" {
			t.Fatalf("endpoint = %+v, err = %v", ep, err)
		}
	})
	t.Run("incomplete llm block keeps the error", func(t *testing.T) {
		path := writeRawConfig(t, `{"llm":{"url":"https://llm.example.test","model":"m"}}`)
		_, err := ResolveEndpoint(path)
		if err == nil || !strings.Contains(err.Error(), "no valid LLM endpoint configured") {
			t.Fatalf("err = %v, want the unresolved-endpoint error", err)
		}
	})
	t.Run("OCR environment wins", func(t *testing.T) {
		t.Setenv("OCR_LLM_URL", "https://llm.example.test")
		t.Setenv("OCR_LLM_TOKEN", "t")
		t.Setenv("OCR_LLM_MODEL", "m")
		ep, err := ResolveEndpoint(missingConfigPath(t))
		if err != nil || ep.Source != "OCR environment" {
			t.Fatalf("endpoint = %+v, err = %v", ep, err)
		}
	})
}

func TestResolveWithoutAnyEndpointMentionsClaudeCode(t *testing.T) {
	clearAllEnv(t)
	_, err := ResolveEndpoint(missingConfigPath(t))
	if err == nil {
		t.Fatal("expected an error without any endpoint")
	}
	for _, want := range []string{"no valid LLM endpoint configured", "Claude Code", "ocr config provider"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestClaudeCodeBinaryExported(t *testing.T) {
	bin := fakeClaudeBin(t)
	if got, err := ClaudeCodeBinary(); err != nil || got != bin {
		t.Fatalf("ClaudeCodeBinary() = %q, %v", got, err)
	}
	t.Setenv(envClaudeCodeBin, filepath.Join(t.TempDir(), "missing"))
	if _, err := ClaudeCodeBinary(); err == nil || !strings.Contains(err.Error(), envClaudeCodeBin) {
		t.Fatalf("err = %v", err)
	}
}

func TestClaudeCodeFallbackReport(t *testing.T) {
	clearAllEnv(t)
	if _, ok := ClaudeCodeFallback(); ok {
		t.Fatal("fallback reported without a reachable claude CLI")
	}
	fakeClaudeBin(t)
	ep, ok := ClaudeCodeFallback()
	if !ok || ep.Provider != "claude-code" || ep.Model != "opus" || ep.Source != envClaudeCodeBin {
		t.Fatalf("ClaudeCodeFallback() = %+v, %v", ep, ok)
	}
}

func TestLegacyLlmBlockStarted(t *testing.T) {
	for body, want := range map[string]bool{
		`{"llm":{"auth_token_cmd":"pass show llm"}}`: true,
		`{"llm":{"timeout_sec":30}}`:                 false,
		`{"llm":`:                                    false,
	} {
		if got := legacyLlmBlockStarted(writeRawConfig(t, body)); got != want {
			t.Errorf("legacyLlmBlockStarted(%s) = %v, want %v", body, got, want)
		}
	}
	if legacyLlmBlockStarted(missingConfigPath(t)) {
		t.Error("a missing file has no llm block")
	}
}
