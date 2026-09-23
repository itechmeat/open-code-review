// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"github.com/alibaba/open-code-review/internal/config/rules"
	"github.com/alibaba/open-code-review/internal/model"
)

// PathSelection answers, from the path alone, whether review would select a
// file, through the same gates selectFiles applies so the two cannot disagree.
// Gates that need the diff itself (binary content, size) are not covered.
func PathSelection(path string, filter *rules.FileFilter) ExcludeReason {
	a := &Agent{args: Args{FileFilter: filter}}
	return a.whyExcluded(model.Diff{OldPath: path, NewPath: path})
}
