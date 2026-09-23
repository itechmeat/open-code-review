// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"fmt"
	"io/fs"

	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alibaba/open-code-review/internal/pathutil"
)

const noMatches = "No matches found"

// regexLike spots search texts written as regular expressions. Models send
// "a|b" without use_perl_regexp, and a literal search for it finds nothing.
var regexLike = regexp.MustCompile(`\||\.\*|\.\+|\\[bswdBSWD]|^\^|\$$`)

// searchWithDiagnostics runs the search and, when it finds nothing, says why:
// a regex-looking literal is retried as a regex, and file_patterns that match
// no file are named, so "No matches found" is never mistaken for "missing".
func (p *CodeSearchProvider) searchWithDiagnostics(ctx context.Context, text string, caseSensitive, usePerl bool, patterns []string) (string, error) {
	result, err := p.search(ctx, text, caseSensitive, usePerl, patterns)
	if err != nil || result != noMatches {
		return result, err
	}
	if !usePerl && regexLike.MatchString(text) {
		if retried, rerr := p.search(ctx, text, caseSensitive, true, patterns); rerr == nil && retried != noMatches {
			return "Note: search_text matched nothing literally; results below treat it as a regular expression.\n" + retried, nil
		}
	}
	if len(patterns) == 0 {
		return result, nil
	}
	var empty []string
	for _, pattern := range patterns {
		if !p.patternMatchesFiles(ctx, pattern) {
			empty = append(empty, pattern)
		}
	}
	if len(empty) == 0 {
		return noMatches + " in the files that match file_patterns.", nil
	}
	return fmt.Sprintf("%s. These file_patterns match no file: %s. Check the paths; installed dependency sources under node_modules/, vendor/ and site-packages/ are searchable.",
		noMatches, strings.Join(empty, ", ")), nil
}

func (p *CodeSearchProvider) search(ctx context.Context, text string, caseSensitive, usePerl bool, patterns []string) (string, error) {
	deps, repo := splitDependencyPatterns(patterns)
	if len(deps) == 0 {
		return p.gitGrep(ctx, text, caseSensitive, usePerl, patterns)
	}
	depResult, err := p.searchDependencies(ctx, text, caseSensitive, usePerl, deps)
	if err != nil || len(repo) == 0 {
		return depResult, err
	}
	repoResult, err := p.gitGrep(ctx, text, caseSensitive, usePerl, repo)
	if err != nil {
		return "", err
	}
	return combineSearchResults(repoResult, depResult), nil
}

// patternMatchesFiles asks git which files the pattern selects, with the same
// pathspec rules the search used.
func (p *CodeSearchProvider) patternMatchesFiles(ctx context.Context, pattern string) bool {
	args := []string{"--no-pager", "grep", "-l", "--max-count", "1", "-e", ""}
	switch {
	case isDependencyPath(pattern):
		args = []string{"--no-pager", "grep", "--no-index", "-l", "--max-count", "1", "-e", "", "--", pattern}
	case p.FileReader.Ref != "":
		args = append(args, p.FileReader.Ref, "--", pattern)
	default:
		args = append(args, "--untracked", "--", pattern)
	}
	out, _, err := p.runGitGrep(ctx, args)
	return err == nil && strings.TrimSpace(out) != ""
}

// dependencyFileWalkBudget bounds the file_find fallback; node_modules trees
// can hold hundreds of thousands of files.
const dependencyFileWalkBudget = 5 * time.Second

// findDependencyFiles matches query against file names inside dependency
// directories of the working tree, which git-based listing cannot see.
func (p *FileFindProvider) findDependencyFiles(ctx context.Context, query string, caseSensitive bool) []string {
	root, err := pathutil.CanonicalPath(p.FileReader.RepoDir)
	if err != nil {
		return nil
	}
	if !caseSensitive {
		query = strings.ToLower(query)
	}
	deadline := time.Now().Add(dependencyFileWalkBudget)
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil || time.Now().After(deadline) || len(found) >= fileFindMaxCount {
			return fs.SkipAll
		}
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil || !isDependencyPath(rel) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		cmp := rel
		if !caseSensitive {
			cmp = strings.ToLower(rel)
		}
		if strings.Contains(cmp, query) {
			found = append(found, rel)
		}
		return nil
	})
	return found
}
