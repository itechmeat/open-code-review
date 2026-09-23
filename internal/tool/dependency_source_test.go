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

func TestDependencyPathRejectsEscapes(t *testing.T) {
	for _, path := range []string{"vendor/../.env", "node_modules/x/../../config/secret.yaml", "node_modules/../src/app.ts"} {
		if isDependencyPath(path) {
			t.Errorf("isDependencyPath(%q) = true; a path that climbs out must not count", path)
		}
	}
}

func TestFileReadRefusesSymlinkOutOfDependencies(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	link := filepath.Join(dir, "node_modules", "evil.js")
	if err := os.Symlink(filepath.Join(dir, ".env"), link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	fr := &FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)}
	if got, err := fr.Read(context.Background(), "node_modules/evil.js"); err == nil {
		t.Fatalf("a dependency symlink to a secret was followed: %q", got)
	}
	if _, _, err := fr.ReadLines(context.Background(), "node_modules/evil.js", 1, 5); err == nil {
		t.Fatal("ReadLines followed a dependency symlink to a secret")
	}
}

func TestFileReadDoesNotFallBackAfterCancel(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	fr := &FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := fr.Read(ctx, "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js"); err == nil {
		t.Fatalf("a cancelled read must fail, got %q", got)
	}
}

func TestCodeSearchMixedPatternsSurviveMissingDependencies(t *testing.T) {
	dir := setupTestRepo(t) // no node_modules installed
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got, err := p.Execute(context.Background(), map[string]any{
		"search_text":   "Util",
		"file_patterns": []any{"pkg/", "node_modules/lib/"},
	})
	if err != nil || !strings.Contains(got, "pkg/util.go") {
		t.Fatalf("repo matches must survive a failed dependency search: %q, %v", got, err)
	}
}

func TestCodeSearchDoesNotLeakThroughDependencySymlinks(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	if err := os.Symlink(filepath.Join(dir, ".env"), filepath.Join(dir, "node_modules", "evil.js")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got, err := p.Execute(context.Background(), map[string]any{"search_text": "API_TOKEN", "file_patterns": []any{"node_modules/"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("secret content leaked through a symlink:\n%s", got)
	}
}

func TestFileFindReachesDependencySources(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	p := NewFileFind(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	for _, q := range []string{"MenuTrigger.js", "MenuTrigger"} {
		got, err := p.Execute(context.Background(), map[string]any{"query_name": q})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "node_modules/@base-ui/react/menu/trigger/MenuTrigger.js") {
			t.Errorf("file_find %q missed the dependency file:\n%s", q, got)
		}
	}
	if got, _ := p.Execute(context.Background(), map[string]any{"query_name": ".env"}); strings.Contains(got, ".env") && !strings.Contains(got, "not found") {
		t.Errorf("file_find must not surface ignored non-dependency files:\n%s", got)
	}
}

func TestFileFindReachesNestedDependencyDirs(t *testing.T) {
	dir := setupRepoWithDependencies(t)
	nested := filepath.Join(dir, "packages", "ui", "node_modules", "lib", "Nested.js")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewFileFind(&FileReader{RepoDir: dir, Mode: ModeRange, Ref: getHeadCommit(t, dir)})
	got, err := p.Execute(context.Background(), map[string]any{"query_name": "Nested.js"})
	if err != nil || !strings.Contains(got, "packages/ui/node_modules/lib/Nested.js") {
		t.Fatalf("nested dependency dir missed: %q, %v", got, err)
	}
}
