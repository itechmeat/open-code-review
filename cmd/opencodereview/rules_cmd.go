// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors
// Modified by Sergey Eroshenkov, 2026; see NOTICE.fork.md.

package main

import (
	"fmt"
	"strings"

	"github.com/alibaba/open-code-review/internal/agent"
	"github.com/alibaba/open-code-review/internal/config/rules"
	"github.com/spf13/cobra"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Inspect and debug review rules",
	Long:  "Inspect and debug review rules.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var rulesCheckRepoDir string
var rulesCheckRulePath string

var rulesCheckCmd = &cobra.Command{
	Use:   "check [flags] <file-path>",
	Short: "Show which review rule applies to a given file path",
	Long:  "Show which review rule applies to the given file path, including its source layer and matched pattern.",
	Example: `  ocr rules check src/main/java/com/example/Foo.java
  ocr rules check --rule custom.json src/main/resources/mapper/UserMapper.xml`,
	Args: exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRulesCheck(args[0])
	},
}

func init() {
	addRepoFlag(rulesCheckCmd, &rulesCheckRepoDir)
	rulesCheckCmd.Flags().StringVar(&rulesCheckRulePath, "rule", "", "path to a custom rule JSON file")
	rulesCmd.AddCommand(rulesCheckCmd)
}

func runRulesCheck(filePath string) error {
	resolvedRepo, err := resolveRepoDir(rulesCheckRepoDir)
	if err != nil {
		return err
	}

	resolver, filter, err := rules.NewResolver(resolvedRepo, rulesCheckRulePath, rules.ResolverOptions{})
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	dr, ok := resolver.(rules.DetailResolver)
	if !ok {
		return fmt.Errorf("resolver does not support detail inspection")
	}

	detail := dr.ResolveDetail(filePath)

	sourceLabel := map[string]string{
		"custom":  "Custom (--rule)",
		"project": "Project (.opencodereview/rule.json)",
		"global":  "Global (~/.opencodereview/rule.json)",
		"system":  "System built-in",
	}

	fmt.Printf("File: %s\n", filePath)
	fmt.Printf("Source: %s\n", sourceLabel[detail.Source])
	fmt.Printf("Pattern: %s\n", detail.Pattern)
	if detail.SniffedAs != "" {
		fmt.Printf("Note:    rule selected by file content (%s), not by path alone\n", detail.SniffedAs)
	}
	fmt.Println(reviewSelectionLine(agent.PathSelection(filePath, filter)))
	fmt.Println("Rule:")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println(detail.Rule)
	fmt.Println(strings.Repeat("─", 40))

	return nil
}

// reviewSelectionLine says whether review would pick the file up at all, so a
// matching rule is not mistaken for a promise that the file gets reviewed.
func reviewSelectionLine(reason agent.ExcludeReason) string {
	switch reason {
	case agent.ExcludeNone:
		return "Review:  selected"
	case agent.ExcludeDefaultPath, agent.ExcludeExtension:
		return fmt.Sprintf("Review:  excluded (%s); list the path under \"include\" in a rule file (--rule, .opencodereview/rule.json) to review it", reason)
	case agent.ExcludeUserRule:
		return fmt.Sprintf("Review:  excluded (%s) by an \"exclude\" pattern in your rule files", reason)
	default:
		return fmt.Sprintf("Review:  excluded (%s)", reason)
	}
}

// previewIncludeHint tells the reader of a preview that files skipped by the
// default path or extension gates can be brought back; the reason code alone
// does not say so.
func previewIncludeHint(entries []agent.DiffPreviewEntry) string {
	for _, e := range entries {
		if !e.WillReview && (e.ExcludeReason == agent.ExcludeDefaultPath || e.ExcludeReason == agent.ExcludeExtension) {
			return "  default_path / unsupported_ext files can be reviewed: list them under \"include\" in a rule file (--rule, .opencodereview/rule.json); check with `ocr rules check <path>`"
		}
	}
	return ""
}
