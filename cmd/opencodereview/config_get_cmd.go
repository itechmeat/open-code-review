// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Print a configuration value, or the whole config with secrets masked",
	Example: `  ocr config get provider
  ocr config get providers.claude-code.model
  ocr config get`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := defaultConfigPath()
		if err != nil {
			return err
		}
		key := ""
		if len(args) == 1 {
			key = args[0]
		}
		return runConfigGet(path, key)
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
}

// maskedValue replaces anything credential-shaped; the command exists so that
// scripts and agents can read the config without ever seeing a secret.
const maskedValue = "(set)"

// runConfigGet reads the raw file rather than the typed config so that every
// key upstream adds later is readable without touching this command.
func runConfigGet(path, key string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	root = maskSecrets("", root)

	value := root
	if key != "" {
		for _, part := range strings.Split(key, ".") {
			obj, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%s is not set", key)
			}
			if value, ok = obj[part]; !ok {
				return fmt.Errorf("%s is not set", key)
			}
		}
	}

	if s, ok := value.(string); ok {
		fmt.Println(s)
		return nil
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func maskSecrets(name string, v any) any {
	switch t := v.(type) {
	case map[string]any:
		// Header maps carry credentials under arbitrary names.
		headers := strings.Contains(strings.ToLower(name), "headers")
		for k, child := range t {
			if headers || isSecretName(k) {
				t[k] = maskedValue
				continue
			}
			t[k] = maskSecrets(k, child)
		}
		return t
	case []any:
		for i := range t {
			t[i] = maskSecrets(name, t[i])
		}
		return t
	default:
		if isSecretName(name) {
			return maskedValue
		}
		return v
	}
}

func isSecretName(name string) bool {
	n := strings.ToLower(name)
	for _, suffix := range []string{"key", "key_cmd", "token", "token_cmd", "secret", "password"} {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}
