// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const configGetFixture = `{
  "provider": "z-ai-coding",
  "max_tokens": 120000,
  "providers": {
    "z-ai-coding": {"api_key_cmd": "jq -r .key auth.json", "model": "glm-5.3-flash"},
    "anthropic": {"api_key": "sk-ant-0123456789abcdef", "model": "claude-opus-4-6"}
  },
  "llm": {"auth_token": "tok-0123456789abcdef", "extra_headers": {"Authorization": "Bearer abcdefghijkl", "X-Trace": "on"}},
  "mcp_servers": {"github": {"command": "gh-mcp", "env": {"GITHUB_TOKEN": "ghp_0123456789abcdef", "LOG_LEVEL": "debug"}}}
}`

func writeConfigGetFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(configGetFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func configGet(t *testing.T, path, key string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := runConfigGet(&buf, io.Discard, path, key)
	return strings.TrimSpace(buf.String()), err
}

func TestConfigGetScalars(t *testing.T) {
	path := writeConfigGetFixture(t)
	for key, want := range map[string]string{
		"provider":                          "z-ai-coding",
		"model":                             "glm-5.3-flash",
		"max_tokens":                        "120000",
		"providers.z-ai-coding.model":       "glm-5.3-flash",
		"providers.z-ai-coding.api_key_cmd": "jq -r .key auth.json",
		"providers.anthropic.api_key":       "sk-a***cdef",
		"llm.AuthToken":                     "tok-***cdef",
		"llm.extra_headers.X-Trace":         "on",
	} {
		got, err := configGet(t, path, key)
		if err != nil {
			t.Errorf("get %s: %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("get %s = %q, want %q", key, got, want)
		}
	}
}

func TestConfigGetMasksSecretsInObjects(t *testing.T) {
	path := writeConfigGetFixture(t)
	out, err := configGet(t, path, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sk-ant-0123456789abcdef", "tok-0123456789abcdef", "Bearer abcdefghijkl", "ghp_0123456789abcdef"} {
		if strings.Contains(out, secret) {
			t.Errorf("full config output leaks %q:\n%s", secret, out)
		}
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("full config is not JSON: %v\n%s", err, out)
	}
	for _, keep := range []string{"debug", "jq -r .key auth.json", "120000"} {
		if !strings.Contains(out, keep) {
			t.Errorf("non-secret %q was masked or dropped:\n%s", keep, out)
		}
	}
}

func TestConfigGetModelFallsBackToTopLevelAndCustomProvider(t *testing.T) {
	dir := t.TempDir()
	for body, want := range map[string]string{
		`{"model":"top-model","provider":"x","custom_providers":{"x":{"model":"other"}}}`: "top-model",
		`{"provider":"gw","custom_providers":{"gw":{"model":"gw-model"}}}`:                "gw-model",
	} {
		path := filepath.Join(dir, "c.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := configGet(t, path, "model"); err != nil || got != want {
			t.Errorf("model for %s = %q, %v; want %q", body, got, err, want)
		}
	}
}

func TestConfigGetErrors(t *testing.T) {
	path := writeConfigGetFixture(t)
	if _, err := configGet(t, path, "effort"); err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("unset key: err = %v", err)
	}
	if _, err := configGet(t, path, "provider.name"); err == nil {
		t.Fatal("descending into a string must fail")
	}
	t.Setenv("OCR_CLAUDE_CODE_BIN", filepath.Join(t.TempDir(), "no-claude"))
	if _, err := configGet(t, filepath.Join(t.TempDir(), "missing.json"), "provider"); err == nil || !strings.Contains(err.Error(), "no config file") {
		t.Fatalf("missing file: err = %v", err)
	}
}

func TestIsSecretConfigName(t *testing.T) {
	for name, want := range map[string]bool{
		"api_key": true, "auth_token": true, "Authorization": true, "x-api-key": true,
		"GITHUB_TOKEN": true, "aws_secret_access_key": true, "password": true,
		"api_key_cmd": false, "auth_token_cmd": false, "max_tokens": false, "model": false, "LOG_LEVEL": false,
	} {
		if got := isSecretConfigName(name); got != want {
			t.Errorf("isSecretConfigName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestConfigGetProviderReportsClaudeCodeFallback(t *testing.T) {
	clearEndpointEnv(t)
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCR_CLAUDE_CODE_BIN", bin)

	for name, path := range map[string]string{
		"no config file":        filepath.Join(t.TempDir(), "config.json"),
		"file without provider": writeRawConfigFile(t, `{"max_tokens": 1000}`),
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := runConfigGet(&out, &errOut, path, "provider"); err != nil {
				t.Fatalf("get provider: %v", err)
			}
			if got := strings.TrimSpace(out.String()); got != "claude-code" {
				t.Fatalf("stdout = %q, want claude-code", got)
			}
			if !strings.Contains(errOut.String(), "OCR_CLAUDE_CODE_BIN") {
				t.Fatalf("stderr does not name the source: %q", errOut.String())
			}
		})
	}

	t.Run("configured provider wins", func(t *testing.T) {
		if got, err := configGet(t, writeConfigGetFixture(t), "provider"); err != nil || got != "z-ai-coding" {
			t.Fatalf("get provider = %q, %v", got, err)
		}
	})
	for name, body := range map[string]string{
		"complete llm block": `{"llm":{"url":"https://llm.example.test","auth_token":"t","model":"m"}}`,
		"half llm block":     `{"llm":{"url":"https://llm.example.test"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := configGet(t, writeRawConfigFile(t, body), "provider"); err == nil || !strings.Contains(err.Error(), "not set") {
				t.Fatalf("get provider = %q, %v; want the not-set error", got, err)
			}
		})
	}
	t.Run("OCR environment", func(t *testing.T) {
		t.Setenv("OCR_LLM_URL", "https://llm.example.test")
		t.Setenv("OCR_LLM_TOKEN", "t")
		if got, err := configGet(t, writeRawConfigFile(t, `{}`), "provider"); err == nil || !strings.Contains(err.Error(), "not set") {
			t.Fatalf("get provider = %q, %v; want the not-set error", got, err)
		}
		if _, err := configGet(t, filepath.Join(t.TempDir(), "config.json"), "provider"); err == nil || !strings.Contains(err.Error(), "no config file") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no claude and no file", func(t *testing.T) {
		t.Setenv("OCR_CLAUDE_CODE_BIN", filepath.Join(t.TempDir(), "missing"))
		if _, err := configGet(t, filepath.Join(t.TempDir(), "config.json"), "provider"); err == nil || !strings.Contains(err.Error(), "no config file") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestConfigGetWithoutFileFailsForOtherKeys(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "config.json")
	for _, key := range []string{"", "model", "providers"} {
		if got, err := configGet(t, missing, key); err == nil || !strings.Contains(err.Error(), "no config file") {
			t.Errorf("get %q = %q, %v; want the no-config-file error", key, got, err)
		}
	}
}

func writeRawConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
