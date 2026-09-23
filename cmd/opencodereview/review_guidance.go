// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"strings"

	"github.com/alibaba/open-code-review/internal/config/template"
)

// reviewGuidance extends the main review prompt with habits field runs showed
// missing: hedged "one of them is wrong" findings anchored on the correct
// side, fixes proposed in generated output instead of its source, and empty
// lookups reported as "not available" instead of being retried.
const reviewGuidance = `## Contradictions and generated code
- When two places disagree (code and test, an invariant and a comment, a schema and its generator, a caller and a library), establish which side is wrong before commenting: read the source that decides it, including installed dependency code under node_modules/, vendor/ or site-packages/. Anchor the comment on the side that is wrong. Say that one of them is wrong only when the evidence cannot decide.
- When a generated file is wrong, find the generator or source file that produces it and name that file in the comment; the fix belongs there, not in the generated output. When that source is among the review files, comment on the source.
- An empty tool result is not a fact about the code. Read the tool's explanation (no file matched the pattern, or nothing matched in the files), then try another path, name or search; installed dependency sources are available. Never write a comment whose point is that something could not be checked or is unavailable to you.
- Do not cite checklist items or rule numbers in comments; each comment must stand on its own for a reader who never saw the checklist.`

// filterToolLimitGround adds a removal ground to the review filter: a claim
// about the reviewer's own tools is not a defect in the diff.
const filterToolLimitGround = `### Also remove
A comment whose central claim is that the reviewer could not check something, or that a file, dependency or tool was unavailable. Such claims are about the reviewer's tools, not defects in the diff.`

func appendReviewGuidance(tpl *template.Template) {
	if tpl == nil {
		return
	}
	if tpl.ReviewFilterTask != nil {
		for i := range tpl.ReviewFilterTask.Messages {
			m := &tpl.ReviewFilterTask.Messages[i]
			if m.Role == "system" {
				if !strings.Contains(m.Content, filterToolLimitGround) {
					m.Content = strings.TrimRight(m.Content, "\n") + "\n\n" + filterToolLimitGround
				}
				break
			}
		}
	}
	for i := range tpl.MainTask.Messages {
		m := &tpl.MainTask.Messages[i]
		if m.Role != "system" {
			continue
		}
		if !strings.Contains(m.Content, reviewGuidance) {
			m.Content = strings.TrimRight(m.Content, "\n") + "\n\n" + reviewGuidance
		}
		return
	}
}

// reviewDedupRootCause widens scan's same-claim dedup for a whole review,
// where one defect often surfaces in several files.
const reviewDedupRootCause = `- Also merge comments that share one root cause, for example several generated files that are wrong because of one generator bug, or one defect reported from two files. Put first in members the comment on the file that must be fixed, and write merged_content that names that file and lists the other affected files.`

// reviewDedupConversation is scan's DEDUP_TASK with the root-cause rule
// added; scan's own template is left as it is.
func reviewDedupConversation() *template.LlmConversation {
	tpl, err := template.LoadScanDefault()
	if err != nil || tpl.DedupTask == nil || len(tpl.DedupTask.Messages) == 0 {
		return nil
	}
	conv := &template.LlmConversation{Messages: append([]template.ChatMessage(nil), tpl.DedupTask.Messages...)}
	for i := range conv.Messages {
		if conv.Messages[i].Role == "system" {
			conv.Messages[i].Content = strings.TrimRight(conv.Messages[i].Content, "\n") + "\n" + reviewDedupRootCause
			break
		}
	}
	return conv
}
