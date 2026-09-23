// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"strings"

	"github.com/alibaba/open-code-review/internal/config/template"
)

// reviewGuidance extends the main review prompt with two habits field runs
// showed missing: hedged "one of them is wrong" findings anchored on the
// correct side, and fixes proposed in generated output instead of its source.
const reviewGuidance = `## Contradictions and generated code
- When two places disagree (code and test, an invariant and a comment, a schema and its generator, a caller and a library), establish which side is wrong before commenting: read the source that decides it, including installed dependency code under node_modules/, vendor/ or site-packages/. Anchor the comment on the side that is wrong. Say that one of them is wrong only when the evidence cannot decide.
- When a generated file is wrong, find the generator or source file that produces it and name that file in the comment; the fix belongs there, not in the generated output. When that source is among the review files, comment on the source.`

func appendReviewGuidance(tpl *template.Template) {
	if tpl == nil {
		return
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
