// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The opt-in rule shipped under examples/ must keep loading and must win over
// the built-in key-spelling-only rules for JSON and YAML.
func TestStructuredDataRuleExampleApplies(t *testing.T) {
	repo := initRulesCheckTestRepo(t)
	setRulesCheckRepo(t, repo)
	example, err := filepath.Abs(filepath.Join("..", "..", "examples", "rules", "structured-data.rule.json"))
	if err != nil {
		t.Fatal(err)
	}
	original := rulesCheckRulePath
	rulesCheckRulePath = example
	t.Cleanup(func() { rulesCheckRulePath = original })

	for _, path := range []string{"contracts/button.contract.json", "contracts/button.yaml", "ci/config.yml"} {
		got := captureStdout(t, func() {
			if err := runRulesCheck(path); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(got, "Source: Custom (--rule)") || !strings.Contains(got, "enum") {
			t.Errorf("%s did not pick up the example rule:\n%s", path, got)
		}
	}
}
