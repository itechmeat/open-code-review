// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"github.com/alibaba/open-code-review/internal/config/rules"
	"github.com/spf13/cobra"
)

func addScopeFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "path", "", "comma-separated repo-relative directories, files or globs; review only changed files under them")
}

// applyCLIScope restricts review to the --path entries (already split).
func applyCLIScope(cc *commonContext, entries []string) {
	if len(entries) == 0 {
		return
	}
	if cc.FileFilter == nil {
		cc.FileFilter = &rules.FileFilter{}
	}
	cc.FileFilter.Scope = append(cc.FileFilter.Scope, entries...)
}

// splitOutsideBraces splits on commas that are not inside a {a,b} group, so a
// brace glob survives a comma-separated flag.
func splitOutsideBraces(raw string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range raw {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, raw[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, raw[start:])
}
