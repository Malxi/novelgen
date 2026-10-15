package models

import "testing"

func TestIsAgentSDKProviderRecognizesAliases(t *testing.T) {
	cases := map[string]bool{
		"claude":     true,
		"Claude":     true,
		"  CLAUDE  ": true,
		"claude_sdk": true,
		"agent":      true,
		"openai":     false,
		"ollama":     false,
		"":           false,
	}
	for provider, want := range cases {
		if got := IsAgentSDKProvider(provider); got != want {
			t.Errorf("IsAgentSDKProvider(%q) = %v, want %v", provider, got, want)
		}
	}
}
