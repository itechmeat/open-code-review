// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
	"github.com/alibaba/open-code-review/internal/session"
)

type scriptedGroupingClient struct {
	replies []string
	reqs    []llm.ChatRequest
}

func (c *scriptedGroupingClient) CompletionsWithCtx(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.reqs = append(c.reqs, req)
	reply := c.replies[len(c.reqs)-1]
	return &llm.ChatResponse{Choices: []llm.Choice{{Message: llm.ResponseMessage{Role: "assistant", Content: &reply}}}}, nil
}

func groupingTask() *template.LlmConversation {
	return &template.LlmConversation{Messages: []template.ChatMessage{
		{Role: "system", Content: "Group files."},
		{Role: "user", Content: "{{file_list}}"},
	}}
}

// A field run lost its grouping to "Mapping the files:\n[...]".
func TestParseGroupingResponseToleratesSurroundingProse(t *testing.T) {
	diffs := []model.Diff{{NewPath: "a.go"}, {NewPath: "b.go"}}
	groups, err := parseGroupingResponse("Mapping the files:\n[{\"label\":\"g\",\"files\":[0,1]}]\nDone.", diffs)
	if err != nil || len(groups) != 1 || len(groups[0].Diffs) != 2 {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
}

func TestCallGroupingLLMRetriesOnceWithJSONReminder(t *testing.T) {
	diffs := []model.Diff{{NewPath: "a.go"}, {NewPath: "b.go"}}
	client := &scriptedGroupingClient{replies: []string{"Mapping the files is easy.", `[{"label":"g","files":[0,1]}]`}}
	groups, _, err := callGroupingLLM(context.Background(), diffs, client, "m", groupingTask(), 1000, nil)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
	if len(client.reqs) != 2 {
		t.Fatalf("want a single retry, got %d calls", len(client.reqs))
	}
	last := client.reqs[1].Messages[len(client.reqs[1].Messages)-1]
	if last.Role != "user" || !strings.Contains(last.ExtractText(), "JSON") {
		t.Errorf("retry must end with a JSON-only reminder, got %+v", last)
	}
}

func TestCallGroupingLLMKeepsRawReplyOnFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	repo := t.TempDir()
	sess := session.New(repo, "main", "m", session.SessionOptions{})
	diffs := []model.Diff{{NewPath: "a.go"}, {NewPath: "b.go"}}
	client := &scriptedGroupingClient{replies: []string{"Mapping, part one.", "Mapping, part two."}}
	if _, _, err := callGroupingLLM(context.Background(), diffs, client, "m", groupingTask(), 1000,
		&groupingSessionOpts{session: sess}); err == nil {
		t.Fatal("expected a parse error after the retry")
	}
	if err := sess.Finalize(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(home, ".opencodereview", "*", "*", sess.SessionID+".jsonl"))
	if len(files) != 1 {
		all, _ := filepath.Glob(filepath.Join(home, "*", "*", "*"))
		t.Fatalf("session file not found: %v (have %v, persist %v)", files, all, sess.HasPersistence())
	}
	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), "Mapping, part two.") {
		t.Error("the unparsable grouping reply must be kept in the session for diagnosis")
	}
}

func TestGroupDiffsFlagsFallbackToPerFile(t *testing.T) {
	var diffs []model.Diff
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		diffs = append(diffs, model.Diff{NewPath: name, Insertions: 300})
	}
	tpl, err := template.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	client := &scriptedGroupingClient{replies: []string{"no json", "still no json"}}
	res := groupDiffs(context.Background(), diffs, client, "m", *tpl, 1_000_000, nil)
	if !res.fallback || len(res.groups) != len(diffs) {
		t.Fatalf("fallback = %v, groups = %d", res.fallback, len(res.groups))
	}
}
