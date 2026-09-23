// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rules

import (
	"strings"
	"testing"
)

// Test files used to get the plain language checklist once a user included
// them; they now get one about what makes a test worth having.
func TestBuiltinRuleForTests(t *testing.T) {
	r, _, err := NewResolver(t.TempDir(), "", ResolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dr := r.(DetailResolver)
	for path, wantTests := range map[string]bool{
		"src/input.contract.test.ts":  true,
		"src/button.spec.tsx":         true,
		"src/__tests__/util.js":       true,
		"internal/llm/client_test.go": true,
		"tests/test_parser.py":        true,
		"pkg/parser_test.py":          true,
		"src/input.ts":                false,
		"internal/llm/client.go":      false,
		"pkg/parser.py":               false,
	} {
		if got := strings.Contains(dr.ResolveDetail(path).Rule, "only worth having if it fails"); got != wantTests {
			t.Errorf("%s: tests rule = %v, want %v", path, got, wantTests)
		}
	}
}
