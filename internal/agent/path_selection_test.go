// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"testing"

	"github.com/alibaba/open-code-review/internal/config/rules"
	"github.com/alibaba/open-code-review/internal/model"
)

func TestPathSelectionScope(t *testing.T) {
	f := &rules.FileFilter{Scope: []string{"packages/ui-kit"}, Include: []string{"**/*.test.ts"}}
	if got := PathSelection("services/api/main.go", f); got != model.ExcludeOutOfScope {
		t.Errorf("outside scope = %q, want %q", got, model.ExcludeOutOfScope)
	}
	// Scope restricts before include admits: an include pattern cannot pull a
	// file outside --path back into the review.
	if got := PathSelection("services/api/a.test.ts", f); got != model.ExcludeOutOfScope {
		t.Errorf("included but outside scope = %q", got)
	}
	if got := PathSelection("packages/ui-kit/a.test.ts", f); got != ExcludeNone {
		t.Errorf("included inside scope = %q", got)
	}
	if got := PathSelection("packages/ui-kit/b.test.js", &rules.FileFilter{Scope: []string{"packages/ui-kit"}}); got != ExcludeDefaultPath {
		t.Errorf("default_path inside scope = %q", got)
	}
}
