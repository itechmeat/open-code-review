// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"strings"
	"testing"
)

func TestResolveModelOverrideAcceptsConfiguredModels(t *testing.T) {
	clearAllEnv(t)
	path := writeRawConfig(t, `{"provider":"anthropic","model":"claude-next-top","providers":{"anthropic":{"api_key":"k","model":"claude-next-entry"}}}`)

	for _, model := range []string{"claude-next-entry", "claude-next-top", "claude-opus-5"} {
		ep, err := ResolveEndpointWithModelOverride(path, model)
		if err != nil || ep.Model != model {
			t.Errorf("--model %s: endpoint = %+v, err = %v", model, ep, err)
		}
	}
}

func TestResolveModelOverrideCustomProviderAcceptsConfiguredModels(t *testing.T) {
	clearAllEnv(t)
	path := writeRawConfig(t, `{"provider":"gw","model":"gw-top","custom_providers":{"gw":{"api_key":"k","url":"https://gw.example/v1","protocol":"openai","model":"gw-entry","models":["gw-listed"]}}}`)

	for _, model := range []string{"gw-entry", "gw-top", "gw-listed"} {
		ep, err := ResolveEndpointWithModelOverride(path, model)
		if err != nil || ep.Model != model {
			t.Errorf("--model %s: endpoint = %+v, err = %v", model, ep, err)
		}
	}
	if _, err := ResolveEndpointWithModelOverride(path, "gw-unknown"); err == nil || !strings.Contains(err.Error(), "is not available") {
		t.Fatalf("unknown model: err = %v", err)
	}
}
