// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rules

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsOutOfScope reports whether path falls outside every Scope entry. An entry
// is a directory (everything below it), a file, or a glob; matching is
// case-insensitive like the include/exclude patterns. An empty Scope restricts
// nothing.
func (f *FileFilter) IsOutOfScope(path string) bool {
	if len(f.Scope) == 0 {
		return false
	}
	lowerPath := strings.ToLower(path)
	for _, entry := range f.Scope {
		for _, p := range expandBraces(strings.TrimSuffix(strings.ToLower(entry), "/")) {
			if lowerPath == p || strings.HasPrefix(lowerPath, p+"/") {
				return false
			}
			if matched, _ := doublestar.Match(p, lowerPath); matched {
				return false
			}
		}
	}
	return true
}
