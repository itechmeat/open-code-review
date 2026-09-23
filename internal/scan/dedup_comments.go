// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package scan

import (
	"context"
	"errors"
	"strings"

	"github.com/alibaba/open-code-review/internal/config/template"
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/model"
)

// DedupComments runs one DEDUP_TASK call over comments and returns the merged
// set, so a diff review can collapse findings that different file groups
// reported about the same problem. It is best effort: on any error, or when
// the groups fail to account for every comment exactly once, the originals
// come back unchanged together with the error.
func DedupComments(ctx context.Context, client llm.LLMClient, modelName string,
	conv *template.LlmConversation, comments []model.LlmComment, maxTokens int) ([]model.LlmComment, *llm.UsageInfo, error) {
	payload := buildDedupCommentsJSON(comments)
	messages := make([]llm.Message, 0, len(conv.Messages))
	for _, m := range conv.Messages {
		messages = append(messages, llm.NewTextMessage(m.Role, strings.ReplaceAll(m.Content, "{{batch_comments}}", payload)))
	}
	resp, err := client.CompletionsWithCtx(ctx, llm.ChatRequest{Model: modelName, Messages: messages, MaxTokens: maxTokens})
	if err != nil {
		return comments, nil, err
	}
	deduped, ok := applyDedupGroups(resp.Content(), comments)
	if !ok {
		return comments, resp.Usage, errors.New("dedup groups did not account for every comment")
	}
	return deduped, resp.Usage, nil
}
