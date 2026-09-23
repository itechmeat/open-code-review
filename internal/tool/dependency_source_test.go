// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoWithDependencies adds an ignored node_modules tree and an ignored
// .env next to the committed sources, the layout of any JS project.
func setupRepoWithDependencies(t *testing.T) string {
	t.Helper()
	dir := setupTestRepo(t)
	files := map[string]string{
		".gitignore": "node_modules/\n.env\n",
		"node_modules/@base-ui/react/menu/trigger/MenuTrigger.js": "export function MenuTrigger() {\n  const openOnHover = false;\n}\n",
		".env": "API_TOKEN=secret\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// In range mode git grep only sees the ref's tree, so a search into an ignored
// dependency directory used to answer "No matches found" and the model took
// that as evidence the code does not exist.
func TestCodeSearchReachesDependencySources(t *testing.T) {
	for _, mode := range []struct {
		name string
		fr   func(dir string) *FileReader
	}{
		{"range", func(dir string) *FileReader {
			return &FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)}
		}},
		{"workspace", func(dir string) *FileReader { return &FileReader{RepoDir: dir, Mode: ModeWorkspace} }},
	} {
		t.Run(mode.name, func(t *testing.T) {
			dir := setupRepoWithDependencies(t)
			p := NewCodeSearch(mode.fr(dir))
			got, err := p.Execute(context.Background(), map[string]any{
				"search_text":   "openOnHover",
				"file_patterns": []any{"node_modules/@base-ui/react/menu/trigger/"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js") || !strings.Contains(got, "2|") {
				t.Errorf("dependency source not searched:\n%s", got)
			}
		})
	}
}

func TestCodeSearchKeepsRepoPatternsOnGitGrep(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got, err := p.Execute(context.Background(), map[string]any{
		"search_text":   "Util",
		"file_patterns": []any{"pkg/", "node_modules/@base-ui/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "pkg/util.go") {
		t.Errorf("repo matches lost when a dependency pattern is present:\n%s", got)
	}
}

func TestFileReadFallsBackToDependencySourcesOnly(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	fr := &FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)}

	got, err := fr.Read(context.Background(), "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js")
	if err != nil || !strings.Contains(got, "openOnHover") {
		t.Fatalf("dependency source unreadable in range mode: %q, %v", got, err)
	}
	lines, total, err := fr.ReadLines(context.Background(), "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js", 2, 1)
	if err != nil || total != 4 || len(lines) != 1 || !strings.Contains(lines[0], "openOnHover") {
		t.Fatalf("ReadLines = %q, %d, %v", lines, total, err)
	}
	// Other ignored files, secrets above all, must stay out of reach.
	if got, err := fr.Read(context.Background(), ".env"); err == nil {
		t.Fatalf("an ignored non-dependency file was read: %q", got)
	}
}

func TestIsDependencyPath(t *testing.T) {
	for path, want := range map[string]bool{
		"node_modules/x/index.js":                 true,
		"packages/ui/node_modules/y/a.js":         true,
		"vendor/github.com/a/b.go":                true,
		".venv/lib/python3.12/site-packages/p.py": true,
		"src/node_modules_helper.ts":              false,
		"node_modules/.env":                       false,
		"src/app.ts":                              false,
	} {
		if got := isDependencyPath(path); got != want {
			t.Errorf("isDependencyPath(%q) = %v, want %v", path, got, want)
		}
	}
}
