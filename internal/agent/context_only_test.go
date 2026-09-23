// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/model"
)

// A changed test file excluded by default_path used to vanish from the prompt,
// so the model could not tell that the matching test changed too.
func TestChangeFilesListExcludedChangesAsContext(t *testing.T) {
	a := New(Args{})
	a.diffs = []model.Diff{{NewPath: "input.ts", OldPath: "input.ts", Insertions: 2}}
	a.rememberContextOnly([]fileDecision{
		{Diff: model.Diff{NewPath: "input.ts", OldPath: "input.ts"}, Reason: ExcludeNone},
		{Diff: model.Diff{NewPath: "input.test.ts", OldPath: "input.test.ts", Insertions: 9}, Reason: ExcludeDefaultPath},
		{Diff: model.Diff{NewPath: "other/x.go", OldPath: "other/x.go", Insertions: 1}, Reason: model.ExcludeOutOfScope},
		{Diff: model.Diff{NewPath: ".env", OldPath: ".env", Insertions: 1}, Reason: ExcludeSecret},
		{Diff: model.Diff{NewPath: "logo.png", OldPath: "logo.png", IsBinary: true}, Reason: ExcludeBinary},
	})

	got := a.buildChangeFilesExceptGroup([]model.Diff{{NewPath: "input.ts", OldPath: "input.ts"}})
	for _, want := range []string{
		"MODIFIED   input.test.ts (+9/-0) [not under review]",
		"MODIFIED   other/x.go (+1/-0) [not under review]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, hidden := range []string{".env", "logo.png"} {
		if strings.Contains(got, hidden) {
			t.Errorf("%s must stay out of the prompt:\n%s", hidden, got)
		}
	}
}
