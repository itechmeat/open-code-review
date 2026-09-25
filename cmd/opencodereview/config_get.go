// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Print a configuration value",
	Long: `Print a value from ~/.opencodereview/config.json, addressed with the same
dotted keys as "ocr config set". Strings print as plain text, everything else
as JSON; with no key the whole file is printed. "model" reports the active
provider's model. Secrets (api_key, auth_token, token-like headers and env
values) are always masked.`,
	Example: "  ocr config get provider\n  ocr config get model\n  ocr config get providers.anthropic\n  ocr config get",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := defaultConfigPath()
		if err != nil {
			return err
		}
		key := ""
		if len(args) == 1 {
			key = args[0]
		}
		return runConfigGet(cmd.OutOrStdout(), configPath, key)
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
}

func runConfigGet(w io.Writer, configPath, key string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no config file at %s; run `ocr config provider` first", configPath)
		}
		return fmt.Errorf("read config: %w", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse config %s: %w", configPath, err)
	}

	value, found, secret := lookupConfigValue(root, key)
	if !found {
		return fmt.Errorf("config key %q is not set", key)
	}
	if secret {
		if s, ok := value.(string); ok {
			value = maskKey(s)
		}
	}
	value = maskConfigSecrets(value)

	if s, ok := value.(string); ok {
		_, err := fmt.Fprintln(w, s)
		return err
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}

// lookupConfigValue resolves a dotted key against the raw config document. It
// returns whether the key was found and whether its own name marks a secret.
//
// Segments match JSON names exactly first, then ignoring case and
// underscores, so the Go-style spellings "ocr config set" accepts
// (llm.AuthToken) read back too. Map keys such as provider names never contain
// dots, so a plain split is unambiguous.
func lookupConfigValue(root map[string]any, key string) (any, bool, bool) {
	if key == "" {
		return root, true, false
	}
	if key == "model" {
		if v, ok := activeProviderModel(root); ok {
			return v, true, false
		}
	}
	var cur any = root
	segments := strings.Split(key, ".")
	for _, seg := range segments {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false, false
		}
		v, ok := lookupField(obj, seg)
		if !ok {
			return nil, false, false
		}
		cur = v
	}
	return cur, true, isSecretConfigName(segments[len(segments)-1])
}

func lookupField(obj map[string]any, name string) (any, bool) {
	if v, ok := obj[name]; ok {
		return v, true
	}
	want := normalizeConfigName(name)
	for k, v := range obj {
		if normalizeConfigName(k) == want {
			return v, true
		}
	}
	return nil, false
}

// activeProviderModel mirrors "ocr config set model": with a provider set, the
// model lives on that provider's entry rather than at the top level.
func activeProviderModel(root map[string]any) (any, bool) {
	if m, ok := root["model"].(string); ok && m != "" {
		return m, true
	}
	provider, _ := root["provider"].(string)
	if provider == "" {
		return nil, false
	}
	for _, section := range []string{"providers", "custom_providers"} {
		entries, _ := root[section].(map[string]any)
		entry, _ := entries[provider].(map[string]any)
		if m, ok := entry["model"].(string); ok && m != "" {
			return m, true
		}
	}
	return nil, false
}

func normalizeConfigName(name string) string {
	return strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(name))
}

// isSecretConfigName reports whether a field, header or env name holds a
// credential. *_cmd fields hold the command that produces one, not the secret.
func isSecretConfigName(name string) bool {
	n := normalizeConfigName(name)
	if strings.HasSuffix(n, "cmd") {
		return false
	}
	if strings.Contains(n, "secret") || strings.Contains(n, "authorization") {
		return true
	}
	// Suffixes rather than substrings, so max_tokens stays readable.
	for _, suffix := range []string{"apikey", "token", "password", "credential", "credentials", "accesskey", "privatekey"} {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}

// maskConfigSecrets returns v with every string under a secret-looking name
// masked, at any depth (provider keys, extra_headers, MCP env and headers).
func maskConfigSecrets(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if s, ok := val.(string); ok && isSecretConfigName(k) {
				out[k] = maskKey(s)
				continue
			}
			out[k] = maskConfigSecrets(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = maskConfigSecrets(val)
		}
		return out
	default:
		return v
	}
}
