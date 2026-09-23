// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import "github.com/alibaba/open-code-review/internal/model"

// rememberContextOnly keeps the changed files that selection left out of the
// review (tests and generated paths, other extensions, user excludes, --path,
// size) so the prompt can still say they changed; file_read_diff can already
// read them. Secret paths, provider directories and binaries stay unnamed.
func (a *Agent) rememberContextOnly(decisions []fileDecision) {
	a.contextOnly = nil
	for _, dec := range decisions {
		switch dec.Reason {
		case ExcludeDefaultPath, ExcludeExtension, ExcludeUserRule, ExcludeTooLarge, model.ExcludeOutOfScope:
			if !dec.Diff.IsBinary {
				a.contextOnly = append(a.contextOnly, dec.Diff)
			}
		}
	}
}
