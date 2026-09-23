// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"reflect"
	"testing"
)

func TestSplitPathsKeepsBraceGroups(t *testing.T) {
	got := splitPaths("packages/{ui-kit,core}/**, docs/*.md ,,template-shell/**")
	want := []string{"packages/{ui-kit,core}/**", "docs/*.md", "template-shell/**"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitPaths = %q, want %q", got, want)
	}
}

func TestReviewAndDelegateHavePathFlag(t *testing.T) {
	for _, name := range []string{"review", "delegate preview"} {
		var args []string
		if name == "review" {
			args = []string{"review"}
		} else {
			args = []string{"delegate", "preview"}
		}
		cmd, _, err := rootCmd.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Flags().Lookup("path") == nil {
			t.Errorf("%s has no --path flag", name)
		}
	}
}

func TestApplyCLIScope(t *testing.T) {
	cc := &commonContext{}
	applyCLIScope(cc, nil)
	if cc.FileFilter != nil {
		t.Error("no --path must leave the filter untouched")
	}
	applyCLIScope(cc, []string{"packages/ui-kit"})
	if cc.FileFilter == nil || !cc.FileFilter.IsOutOfScope("apps/a.ts") || cc.FileFilter.IsOutOfScope("packages/ui-kit/a.ts") {
		t.Errorf("filter = %+v", cc.FileFilter)
	}
}
