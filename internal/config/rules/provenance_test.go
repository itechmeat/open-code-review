// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type canonicalStub struct{ DetailResolver }

func (canonicalStub) Resolve(string) string     { return "" }
func (canonicalStub) CanonicalConfig() []string { return []string{"cfg"} }

func TestWithProvenanceLabelsBuiltinRules(t *testing.T) {
	base, _, err := NewResolver(t.TempDir(), "", ResolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r := WithProvenance(base)
	got := r.Resolve("src/app.ts")
	if !strings.HasPrefix(got, "Checklist source: OpenCodeReview's built-in defaults") || !strings.Contains(got, base.Resolve("src/app.ts")) {
		t.Fatalf("built-in rule not labelled:\n%s", got)
	}
	if dr, ok := r.(DetailResolver); !ok || dr.ResolveDetail("src/app.ts").Source != "system" {
		t.Error("ResolveDetail must still reach the wrapped resolver")
	}
}

func TestWithProvenanceLabelsCustomRules(t *testing.T) {
	rule := filepath.Join(t.TempDir(), "rule.json")
	if err := os.WriteFile(rule, []byte(`{"rules":[{"path":"**/*.ts","rule":"Check enums."}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base, _, err := NewResolver(t.TempDir(), rule, ResolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := WithProvenance(base).Resolve("a.ts"); !strings.HasPrefix(got, "Checklist source: the rule file passed with --rule") || !strings.HasSuffix(got, "Check enums.") {
		t.Errorf("custom rule label = %q", got)
	}
}

func TestWithProvenanceForwardsCanonicalConfig(t *testing.T) {
	// The resume identity hashes CanonicalConfig; wrapping must not hide it.
	r := WithProvenance(canonicalStub{})
	cc, ok := r.(interface{ CanonicalConfig() []string })
	if !ok || !reflect.DeepEqual(cc.CanonicalConfig(), []string{"cfg"}) {
		t.Fatal("CanonicalConfig not forwarded")
	}
	if r.Resolve("x") != "" {
		t.Error("an empty rule must stay empty")
	}
}
