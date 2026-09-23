// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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

// dependencyMatch is a file found under a dependency directory. Stray marks
// a directory whose parent is not part of the reviewed tree (an untracked side
// project), which may hold a different version than the reviewed code uses.
type dependencyMatch struct {
	path  string
	stray bool
}

type dependencyRoot struct {
	dir   string // repo-relative, slash-separated
	stray bool
}

// findDependencyFiles matches query against file names inside dependency
// directories of the working tree, which git-based listing cannot see. It
// also returns the directories it searched, for the not-found message.
func (p *FileFindProvider) findDependencyFiles(ctx context.Context, query string, caseSensitive bool) ([]dependencyMatch, []dependencyRoot) {
	root, err := pathutil.CanonicalPath(p.FileReader.RepoDir)
	if err != nil {
		return nil, nil
	}
	if !caseSensitive {
		query = strings.ToLower(query)
	}
	deadline := time.Now().Add(dependencyFileWalkBudget)
	roots := p.dependencyRoots(ctx)
	walkRoots := roots
	if roots == nil {
		walkRoots = []dependencyRoot{{dir: "."}}
	}
	var matches []dependencyMatch
	for _, r := range walkRoots {
		var found []string
		p.walkDependencyTree(ctx, root, filepath.Join(root, filepath.FromSlash(r.dir)), query, caseSensitive, deadline, &found)
		for _, f := range found {
			matches = append(matches, dependencyMatch{path: f, stray: r.stray})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return !matches[i].stray && matches[j].stray })
	return matches, roots
}

// dependencyRoots lists the ignored dependency directories without entering
// them, so the walk skips the rest of a large repository. It returns nil when
// git cannot answer, and the caller walks the whole tree instead.
func (p *FileFindProvider) dependencyRoots(ctx context.Context) []dependencyRoot {
	out, err := p.git(ctx, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		return nil
	}
	roots := []dependencyRoot{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		dir := strings.TrimSuffix(line, "/")
		if dir == "" || !dependencyDirs[path.Base(dir)] {
			continue
		}
		roots = append(roots, dependencyRoot{dir: dir, stray: !p.inReviewedTree(ctx, path.Dir(dir))})
	}
	return roots
}

// inReviewedTree reports whether dir holds files of the reviewed tree: at the
// ref in range and commit mode, tracked files in workspace mode.
func (p *FileFindProvider) inReviewedTree(ctx context.Context, dir string) bool {
	if dir == "." || dir == "" {
		return true
	}
	var out string
	var err error
	if ref := p.FileReader.Ref; ref != "" {
		out, err = p.git(ctx, "ls-tree", "--name-only", "--end-of-options", ref, "--", dir)
	} else {
		out, err = p.git(ctx, "ls-files", "--", dir)
	}
	return err == nil && strings.TrimSpace(out) != ""
}

func (p *FileFindProvider) git(ctx context.Context, args ...string) (string, error) {
	if p.FileReader.Runner != nil {
		out, err := p.FileReader.Runner.Output(ctx, p.FileReader.RepoDir, args...)
		return string(out), err
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.FileReader.RepoDir
	out, err := cmd.Output()
	return string(out), err
}

// dependencyFindResult renders the dependency fallback of file_find, or says
// where it looked when nothing matched.
func (p *FileFindProvider) dependencyFindResult(ctx context.Context, query string, caseSensitive bool) string {
	matches, roots := p.findDependencyFiles(ctx, query, caseSensitive)
	if len(matches) > 0 {
		var sb strings.Builder
		sb.WriteString("// Found in installed dependency sources (working tree):")
		for _, m := range matches {
			sb.WriteString("\n" + m.path)
			if m.stray {
				sb.WriteString("  (under an untracked directory, not part of the reviewed tree)")
			}
		}
		return sb.String()
	}
	if len(roots) == 0 {
		return "// The file was not found in the reviewed tree, and no installed dependency directories (node_modules/, vendor/, site-packages/) exist in the working tree"
	}
	dirs := make([]string, len(roots))
	for i, r := range roots {
		dirs[i] = r.dir + "/"
	}
	return "// The file was not found in the reviewed tree or in installed dependency sources (searched: " + strings.Join(dirs, ", ") + ")"
}

func (p *FileFindProvider) walkDependencyTree(ctx context.Context, root, walkRoot, query string, caseSensitive bool, deadline time.Time, found *[]string) {
	_ = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil || time.Now().After(deadline) || len(*found) >= fileFindMaxCount {
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
			*found = append(*found, rel)
		}
		return nil
	})
}
