package config

import "testing"

func TestKeyIsolationAndPrecedence(t *testing.T) {
	for _, key := range []string{"CALLM_API_KEY", "STRAITLY_API_KEY", "OPENAI_API_KEY", "DEEPSEEK_API_KEY", "ZAI_API_KEY", "ZHIPU_API_KEY", "DASHSCOPE_API_KEY", "QWEN_API_KEY", "OLLAMA_API_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-dummy")
	if key, _ := ResolveAPIKey("oa", "", ""); key != "" {
		t.Fatal("unrelated key returned")
	}
	t.Setenv("OPENAI_API_KEY", "openai-dummy")
	if key, _ := ResolveAPIKey("oa", "", ""); key != "openai-dummy" {
		t.Fatal("provider key missing")
	}
	t.Setenv("CALLM_API_KEY", "global-dummy")
	if key, _ := ResolveAPIKey("oa", "", ""); key != "global-dummy" {
		t.Fatal("global precedence")
	}
	t.Setenv("TEST_CALLM_KEY", "named-dummy")
	if key, _ := ResolveAPIKey("oa", "", "TEST_CALLM_KEY"); key != "named-dummy" {
		t.Fatal("named precedence")
	}
	if key, _ := ResolveAPIKey("oa", "explicit-dummy", "TEST_CALLM_KEY"); key != "explicit-dummy" {
		t.Fatal("explicit precedence")
	}
	if _, err := ResolveAPIKey("oa", "", "TEST_MISSING_CALLM_KEY"); err == nil {
		t.Fatal("missing named env accepted")
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	t.Setenv("CALLM_BASE_URL", "")
	t.Setenv("OPENAI_BASE_URL", "http://openai-proxy.invalid/v1")
	t.Setenv("STRAITLY_BASE_URL", "http://straitly-proxy.invalid/v1")
	if ResolveBaseURL("oa", "") != "http://openai-proxy.invalid/v1" || ResolveBaseURL("st", "") != "http://straitly-proxy.invalid/v1" {
		t.Fatal("provider URL lost")
	}
	t.Setenv("CALLM_BASE_URL", "http://global.invalid")
	if ResolveBaseURL("oa", "") != "http://global.invalid" || ResolveBaseURL("oa", "http://explicit.invalid") != "http://explicit.invalid" {
		t.Fatal("URL precedence")
	}
}

func TestOrcaConfiguration(t *testing.T) {
	t.Setenv("CALLM_API_KEY", "")
	t.Setenv("ORCA_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "other-provider")
	if key, _ := ResolveAPIKey("orca", "", ""); key != "" {
		t.Fatal("OpenRouter key leaked to OrcaRouter")
	}
	t.Setenv("ORCA_API_KEY", "orca-dummy")
	for _, tc := range []struct{ direct, env, global, want string }{
		{"", "", "", "orca-dummy"}, {"", "", "global", "global"},
		{"", "ORCA_API_KEY", "global", "orca-dummy"}, {"explicit", "ORCA_API_KEY", "global", "explicit"},
	} {
		t.Setenv("CALLM_API_KEY", tc.global)
		if got, err := ResolveAPIKey("orca", tc.direct, tc.env); err != nil || got != tc.want {
			t.Fatalf("key=%q err=%v", got, err)
		}
	}
	t.Setenv("CALLM_BASE_URL", "")
	t.Setenv("OPENAI_BASE_URL", "http://unrelated.invalid")
	if got := ResolveBaseURL("orca", ""); got != "https://api.orcarouter.ai/v1" {
		t.Fatal(got)
	}
	t.Setenv("CALLM_BASE_URL", "http://proxy.invalid/v1")
	if ResolveBaseURL("orca", "") != "http://proxy.invalid/v1" || ResolveBaseURL("orca", "http://explicit.invalid") != "http://explicit.invalid" {
		t.Fatal("endpoint overrides")
	}
}

func TestKimiConfiguration(t *testing.T) {
	t.Setenv("CALLM_API_KEY", "")
	t.Setenv("KIMI_API_KEY", "")
	t.Setenv("MOONSHOT_API_KEY", "moonshot-dummy")
	if key, _ := ResolveAPIKey("kimi", "", ""); key != "" {
		t.Fatal("Moonshot key leaked to Kimi Code")
	}
	t.Setenv("KIMI_API_KEY", "kimi-dummy")
	t.Setenv("MOONSHOT_API_KEY", "")
	if key, _ := ResolveAPIKey("ms", "", ""); key != "" {
		t.Fatal("Kimi Code key leaked to Moonshot")
	}
	for _, tc := range []struct{ direct, env, global, want string }{
		{"", "", "", "kimi-dummy"}, {"", "", "global", "global"},
		{"", "KIMI_API_KEY", "global", "kimi-dummy"}, {"explicit", "KIMI_API_KEY", "global", "explicit"},
	} {
		t.Setenv("CALLM_API_KEY", tc.global)
		if got, err := ResolveAPIKey("kimi", tc.direct, tc.env); err != nil || got != tc.want {
			t.Fatalf("key=%q err=%v", got, err)
		}
	}
	t.Setenv("CALLM_BASE_URL", "")
	t.Setenv("OPENAI_BASE_URL", "http://unrelated.invalid")
	if got := ResolveBaseURL("kimi", ""); got != "https://api.kimi.com/coding/v1" {
		t.Fatal(got)
	}
	if Presets["kimi"].DefaultModel != "kimi-for-coding" {
		t.Fatal("wrong subscription model")
	}
	if ResolveBaseURL("ms", "") != "https://api.moonshot.cn/v1" {
		t.Fatal("Moonshot endpoint changed")
	}
	t.Setenv("CALLM_BASE_URL", "http://proxy.invalid/v1")
	if ResolveBaseURL("kimi", "") != "http://proxy.invalid/v1" || ResolveBaseURL("kimi", "http://explicit.invalid") != "http://explicit.invalid" {
		t.Fatal("endpoint overrides")
	}
}
