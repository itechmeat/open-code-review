// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rules check used to print the matching rule for files that review then
// silently excluded, e.g. *.test.ts under default_path.
func TestRunRulesCheckReportsDefaultPathExclusion(t *testing.T) {
	repo := initRulesCheckTestRepo(t)
	setRulesCheckRepo(t, repo)

	got := captureStdout(t, func() {
		if err := runRulesCheck("src/input.contract.test.ts"); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(got, "Review:  excluded (default_path)") || !strings.Contains(got, `"include"`) {
		t.Errorf("expected a default_path exclusion with an include hint, got:\n%s", got)
	}

	got = captureStdout(t, func() {
		if err := runRulesCheck("src/input.ts"); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(got, "Review:  selected") {
		t.Errorf("expected an ordinary source file to be selected, got:\n%s", got)
	}
}

func TestRunRulesCheckIncludeOverridesDefaultPath(t *testing.T) {
	repo := initRulesCheckTestRepo(t)
	setRulesCheckRepo(t, repo)
	rule := filepath.Join(t.TempDir(), "rule.json")
	if err := os.WriteFile(rule, []byte(`{"include":["**/*.contract.test.ts"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	original := rulesCheckRulePath
	rulesCheckRulePath = rule
	t.Cleanup(func() { rulesCheckRulePath = original })

	got := captureStdout(t, func() {
		if err := runRulesCheck("src/input.contract.test.ts"); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(got, "Review:  selected") {
		t.Errorf("an include pattern must bring the file back, got:\n%s", got)
	}
}
