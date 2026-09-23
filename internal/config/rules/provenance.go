// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rules

// provenanceLabels names where a checklist came from. Without it the model
// quotes OCR's generic defaults as "the project's policy".
var provenanceLabels = map[string]string{
	"system":  "Checklist source: OpenCodeReview's built-in defaults for this file type, not a policy of this repository. Do not cite this checklist, its source or any project policy in comments; explain each issue on its own merits.",
	"custom":  "Checklist source: the rule file passed with --rule for this review.",
	"project": "Checklist source: this repository's .opencodereview/rule.json.",
	"global":  "Checklist source: the user's global ~/.opencodereview/rule.json.",
}

type provenanceResolver struct {
	base Resolver
}

// WithProvenance prefixes each resolved rule with a line naming its source.
// Resolvers that cannot report a source are returned unchanged.
func WithProvenance(base Resolver) Resolver {
	if _, ok := base.(DetailResolver); !ok {
		return base
	}
	return provenanceResolver{base: base}
}

func (p provenanceResolver) Resolve(path string) string {
	rule := p.base.Resolve(path)
	if rule == "" {
		return ""
	}
	label, ok := provenanceLabels[p.base.(DetailResolver).ResolveDetail(path).Source]
	if !ok {
		return rule
	}
	return label + "\n" + rule
}

func (p provenanceResolver) ResolveDetail(path string) RuleDetail {
	return p.base.(DetailResolver).ResolveDetail(path)
}

// CanonicalConfig forwards the wrapped resolver's config so the resume
// identity hash stays what it was without the labels.
func (p provenanceResolver) CanonicalConfig() []string {
	if c, ok := p.base.(interface{ CanonicalConfig() []string }); ok {
		return c.CanonicalConfig()
	}
	return nil
}
