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
