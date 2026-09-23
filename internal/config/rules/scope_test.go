// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rules

import "testing"

func TestFileFilterIsOutOfScope(t *testing.T) {
	f := &FileFilter{Scope: []string{"packages/ui-kit", "apps/web/", "docs/{guide,api}/**", "README.md"}}
	for path, want := range map[string]bool{
		"packages/ui-kit/src/a.ts":   false,
		"packages/ui-kit":            false,
		"packages/ui-kit-extra/a.ts": true,
		"apps/web/page.tsx":          false,
		"docs/guide/intro.md":        false,
		"docs/api/x.md":              false,
		"docs/blog/x.md":             true,
		"README.md":                  false,
		"Packages/UI-Kit/src/a.ts":   false,
		"services/api/main.go":       true,
	} {
		if got := f.IsOutOfScope(path); got != want {
			t.Errorf("IsOutOfScope(%q) = %v, want %v", path, got, want)
		}
	}
	if (&FileFilter{}).IsOutOfScope("anything.go") {
		t.Error("an empty scope must not restrict anything")
	}
}
