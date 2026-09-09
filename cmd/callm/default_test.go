package main

import "testing"

// The default provider is Poolside; without an explicit preset, the first
// provider whose key is set wins, in the documented priority order.
func TestDefaultPresetDetection(t *testing.T) {
	keys := []string{"POOLSIDE_API_KEY", "ORCA_API_KEY", "STRAITLY_API_KEY", "DEEPSEEK_API_KEY", "OPENROUTER_API_KEY", "KIMI_API_KEY", "ANTHROPIC_API_KEY", "GROQ_API_KEY"}
	cases := []struct {
		name string
		env  map[string]string
		flag string
		want string
	}{
		{"no keys", map[string]string{}, "", "pool"},
		{"unrelated keys only", map[string]string{"GROQ_API_KEY": "g"}, "", "pool"},
		{"poolside", map[string]string{"POOLSIDE_API_KEY": "k"}, "", "pool"},
		{"orcarouter", map[string]string{"ORCA_API_KEY": "k"}, "", "orca"},
		{"straitly", map[string]string{"STRAITLY_API_KEY": "k"}, "", "st"},
		{"deepseek", map[string]string{"DEEPSEEK_API_KEY": "k"}, "", "ds"},
		{"openrouter", map[string]string{"OPENROUTER_API_KEY": "k"}, "", "or"},
		{"kimi", map[string]string{"KIMI_API_KEY": "k"}, "", "kimi"},
		{"priority pool over all", map[string]string{"POOLSIDE_API_KEY": "k", "ORCA_API_KEY": "k", "STRAITLY_API_KEY": "k"}, "", "pool"},
		{"priority orca over straitly", map[string]string{"ORCA_API_KEY": "k", "STRAITLY_API_KEY": "k"}, "", "orca"},
		{"explicit st overrides keys", map[string]string{"POOLSIDE_API_KEY": "k", "STRAITLY_API_KEY": "k"}, "st", "st"},
		{"explicit pool overrides keys", map[string]string{"STRAITLY_API_KEY": "k"}, "pool", "pool"},
		{"claude with only anthropic key", map[string]string{"ANTHROPIC_API_KEY": "k"}, "claude", "ant"},
		{"claude with anthropic and poolside keys", map[string]string{"ANTHROPIC_API_KEY": "k", "POOLSIDE_API_KEY": "k"}, "claude", "ant"},
		{"claude with anthropic and openrouter keys", map[string]string{"ANTHROPIC_API_KEY": "k", "OPENROUTER_API_KEY": "k"}, "claude", "or"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "")
			}
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			var p presetFlags
			switch tc.flag {
			case "st":
				p.stPreset = true
			case "pool":
				p.poolPreset = true
			case "claude":
				p.claudeFlag = true
			}
			if got := p.ResolvePreset(); got != tc.want {
				t.Fatalf("default preset = %q, want %q", got, tc.want)
			}
		})
	}
}
