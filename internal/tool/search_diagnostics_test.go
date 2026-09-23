// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"strings"
	"testing"
)

func runSearch(t *testing.T, p *CodeSearchProvider, args map[string]any) string {
	t.Helper()
	got, err := p.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Field runs sent "a|b" as a literal search 17 times and read the empty
// answers as proof that nothing matched.
func TestCodeSearchRetriesAlternationAsRegex(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})

	got := runSearch(t, p, map[string]any{"search_text": "Hello|Util"})
	if !strings.Contains(got, "hello.go") || !strings.Contains(got, "pkg/util.go") || !strings.Contains(got, "regular expression") {
		t.Errorf("alternation over the repo not retried as a regex:\n%s", got)
	}
	got = runSearch(t, p, map[string]any{"search_text": "delay|openOnHover", "file_patterns": []any{"node_modules/@base-ui/react/menu/trigger/MenuTrigger.js"}})
	if !strings.Contains(got, "MenuTrigger.js") {
		t.Errorf("alternation over a dependency not retried as a regex:\n%s", got)
	}
	// A literal hit is returned as is, without the retry note.
	got = runSearch(t, p, map[string]any{"search_text": "func Hello"})
	if strings.Contains(got, "regular expression") {
		t.Errorf("literal match must not be retried:\n%s", got)
	}
}

func TestCodeSearchSaysWhenPatternsMatchNoFiles(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})

	got := runSearch(t, p, map[string]any{"search_text": "trap-focus", "file_patterns": []any{"node_modules/@base-ui/react/esm/dialog/", "pkg/"}})
	if !strings.Contains(got, "match no file") || !strings.Contains(got, "node_modules/@base-ui/react/esm/dialog/") || strings.Contains(got, "pkg/,") {
		t.Errorf("an empty pattern must be named, the matching one not:\n%s", got)
	}
	got = runSearch(t, p, map[string]any{"search_text": "nonexistentXYZ", "file_patterns": []any{"pkg/"}})
	if !strings.HasPrefix(got, "No matches found") || !strings.Contains(got, "in the files that match") {
		t.Errorf("a real miss must say the files were searched:\n%s", got)
	}
}

// A bare glob such as *.d.ts used to search only the reviewed tree and report
// a plain miss, although the declarations lived under node_modules.
func TestCodeSearchBareGlobFallsBackToDependencies(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got := runSearch(t, p, map[string]any{"search_text": "openOnHover", "file_patterns": []any{"*.js"}})
	if !strings.Contains(got, "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js") || !strings.Contains(got, "installed dependency sources") {
		t.Errorf("bare glob did not fall back to dependency sources:\n%s", got)
	}
}

func TestCodeSearchWithoutPatternsHintsAtDependencies(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got := runSearch(t, p, map[string]any{"search_text": "openOnHover"})
	if !strings.HasPrefix(got, "No matches found") || !strings.Contains(got, "not searched") {
		t.Errorf("an unscoped miss must say dependencies were not searched:\n%s", got)
	}
}
