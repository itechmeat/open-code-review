// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	allowedext "github.com/alibaba/open-code-review/internal/config/allowlist"
	"github.com/alibaba/open-code-review/internal/pathutil"
)

// dependencyDirs hold installed third-party sources. They are git-ignored, so
// git grep and git show cannot see them, yet a reviewer often needs them to
// decide whether a caller or the library is right.
var dependencyDirs = map[string]bool{"node_modules": true, "vendor": true, "site-packages": true}

// isDependencyPath reports whether path lies inside a dependency directory.
// Only such paths may be read from the working tree outside git; other
// ignored files, secrets above all, stay out of reach.
func isDependencyPath(path string) bool {
	path = strings.TrimPrefix(filepath.ToSlash(path), "./")
	if allowedext.IsSecretPath(path) {
		return false
	}
	segments := strings.Split(path, "/")
	for _, s := range segments {
		// A path that climbs out could name any ignored file.
		if s == ".." {
			return false
		}
	}
	for i, s := range segments {
		if dependencyDirs[s] && i < len(segments)-1 {
			return true
		}
	}
	return false
}

// dependencyDiskPath resolves path in the working tree and confirms that,
// after symlinks, it still lies inside a dependency directory; pnpm-style
// links inside node_modules are fine, a link pointing at .env is not.
func (fr *FileReader) dependencyDiskPath(path string) (string, error) {
	full, err := fr.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}
	root, err := pathutil.CanonicalPath(fr.RepoDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || !isDependencyPath(rel) {
		return "", fmt.Errorf("file path %q resolves outside dependency sources", path)
	}
	return full, nil
}

func (fr *FileReader) readDependencyFromDisk(path string) (string, error) {
	full, err := fr.dependencyDiskPath(path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(content), nil
}

func splitDependencyPatterns(patterns []string) (deps, repo []string) {
	for _, p := range patterns {
		if isDependencyPath(p) {
			deps = append(deps, p)
		} else {
			repo = append(repo, p)
		}
	}
	return deps, repo
}

// searchDependencies greps the working tree under dependency paths, ignoring
// .gitignore on purpose, and renders matches like the main search.
func (p *CodeSearchProvider) searchDependencies(ctx context.Context, searchText string, caseSensitive, usePerlRegexp bool, patterns []string) (string, error) {
	args := []string{"--no-pager", "-c", "core.quotepath=false", "grep", "--no-index", "-n", "--no-color",
		"--max-count", fmt.Sprintf("%d", gitGrepMaxCount+1)}
	if !caseSensitive {
		args = append(args, "-i")
	}
	if usePerlRegexp {
		args = append(args, "-P")
	} else {
		args = append(args, "-F")
	}
	args = append(args, "-e", searchText, "--")
	args = append(args, patterns...)

	out, _, err := p.runGitGrep(ctx, args)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || out != "" {
			return "", fmt.Errorf("search dependency sources: %w", err)
		}
	}

	var order []string
	byFile := map[string][]string{}
	count := 0
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 || !isDependencyPath(parts[0]) || count >= gitGrepMaxCount {
			continue
		}
		if _, seen := byFile[parts[0]]; !seen {
			order = append(order, parts[0])
		}
		byFile[parts[0]] = append(byFile[parts[0]], parts[1]+"|"+parts[2])
		count++
	}
	if len(order) == 0 {
		return "No matches found", nil
	}
	var sb strings.Builder
	sb.WriteString("Note: matches below come from installed dependency sources in the working tree, not from the reviewed commit.\n")
	for _, f := range order {
		fmt.Fprintf(&sb, "File: %s\nMatch lines: %d\n%s\n\n", f, len(byFile[f]), strings.Join(byFile[f], "\n"))
	}
	return sb.String(), nil
}

func combineSearchResults(repo, deps string) string {
	switch {
	case repo == "No matches found":
		return deps
	case deps == "No matches found":
		return repo
	default:
		return repo + "\n" + deps
	}
}
