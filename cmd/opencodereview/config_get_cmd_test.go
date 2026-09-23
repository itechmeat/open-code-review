// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGetConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"provider":"z-ai-coding","providers":{"z-ai-coding":{"api_key":"sk-secret-1234","model":"glm-5","timeout_sec":300,"extra_headers":{"X-Api-Key":"hdr-secret"}},"claude-code":{"model":"sonnet"}},"custom_providers":{"nous":{"api_key_cmd":"pass show nous","url":"https://x"}},"llm":{"auth_token":"tok-secret","auth_header":"x-api-key"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigGetScalar(t *testing.T) {
	path := writeGetConfig(t)
	for key, want := range map[string]string{
		"provider":                          "z-ai-coding",
		"providers.claude-code.model":       "sonnet",
		"providers.z-ai-coding.timeout_sec": "300",
		"llm.auth_header":                   "x-api-key",
	} {
		var err error
		out := captureStdout(t, func() { err = runConfigGet(path, key) })
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if strings.TrimSpace(out) != want {
			t.Errorf("get %s = %q, want %q", key, strings.TrimSpace(out), want)
		}
	}
}

func TestConfigGetMasksSecrets(t *testing.T) {
	path := writeGetConfig(t)
	var err error
	out := captureStdout(t, func() { err = runConfigGet(path, "") })
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"sk-secret-1234", "tok-secret", "pass show nous", "hdr-secret"} {
		if strings.Contains(out, secret) {
			t.Errorf("secret %q leaked:\n%s", secret, out)
		}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("whole-config output must be JSON: %v\n%s", err, out)
	}
	if parsed["provider"] != "z-ai-coding" {
		t.Errorf("provider = %v", parsed["provider"])
	}

	out = captureStdout(t, func() { err = runConfigGet(path, "providers.z-ai-coding.api_key") })
	if err != nil || strings.Contains(out, "sk-secret") || !strings.Contains(out, "set") {
		t.Errorf("masked scalar = %q, err %v", out, err)
	}
}

func TestConfigGetMissingKey(t *testing.T) {
	path := writeGetConfig(t)
	if err := runConfigGet(path, "providers.nope.model"); err == nil || !strings.Contains(err.Error(), "not set") {
		t.Fatalf("err = %v", err)
	}
	if err := runConfigGet(filepath.Join(t.TempDir(), "absent.json"), "provider"); err == nil {
		t.Fatal("missing config file must be an error")
	}
}

func TestConfigGetIsRegistered(t *testing.T) {
	cmd, _, err := configCmd.Find([]string{"get"})
	if err != nil || cmd.Name() != "get" {
		t.Fatalf("ocr config get not registered: %v", err)
	}
}
