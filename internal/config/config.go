package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderPreset defines configuration defaults for a known provider.
type ProviderPreset struct {
	Name         string
	BaseURL      string
	DefaultModel string
	KeyEnv       string
}

var Presets = map[string]ProviderPreset{
	"st": {
		Name:         "Straitly",
		BaseURL:      "https://api.straitly.ai/v1",
		DefaultModel: "deepseek/deepseek-v4-flash-0731",
		KeyEnv:       "STRAITLY_API_KEY",
	},
	"or": {
		Name:         "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		DefaultModel: "deepseek/deepseek-v4-flash-0731",
		KeyEnv:       "OPENROUTER_API_KEY",
	},
	"orca": {
		Name:         "OrcaRouter",
		BaseURL:      "https://api.orcarouter.ai/v1",
		DefaultModel: "orcarouter/auto",
		KeyEnv:       "ORCA_API_KEY",
	},
	"ds": {
		Name:         "DeepSeek Direct",
		BaseURL:      "https://api.deepseek.com",
		DefaultModel: "deepseek-chat",
		KeyEnv:       "DEEPSEEK_API_KEY",
	},
	"ant": {
		Name:         "Anthropic Direct",
		BaseURL:      "https://api.anthropic.com/v1",
		DefaultModel: "claude-sonnet-4-6",
		KeyEnv:       "ANTHROPIC_API_KEY",
	},
	"ms": {
		Name:         "Moonshot Kimi",
		BaseURL:      "https://api.moonshot.cn/v1",
		DefaultModel: "moonshot-v1-auto",
		KeyEnv:       "MOONSHOT_API_KEY",
	},
	"kimi": {
		Name:         "Kimi Code (subscription)",
		BaseURL:      "https://api.kimi.com/coding/v1",
		DefaultModel: "kimi-for-coding",
		KeyEnv:       "KIMI_API_KEY",
	},
	"zai": {
		Name:         "Zhipu AI (GLM)",
		BaseURL:      "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel: "glm-4-flash",
		KeyEnv:       "ZAI_API_KEY",
	},
	"qw": {
		Name:         "Alibaba DashScope (Qwen)",
		BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1",
		DefaultModel: "qwen-plus",
		KeyEnv:       "DASHSCOPE_API_KEY",
	},
	"oa": {
		Name:         "OpenAI Direct",
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
		KeyEnv:       "OPENAI_API_KEY",
	},
	"groq": {
		Name:         "Groq OSS",
		BaseURL:      "https://api.groq.com/openai/v1",
		DefaultModel: "llama-3.3-70b-versatile",
		KeyEnv:       "GROQ_API_KEY",
	},
	"ollama": {
		Name:         "Ollama Local",
		BaseURL:      "http://localhost:11434/v1",
		DefaultModel: "deepseek-r1",
		KeyEnv:       "OLLAMA_API_KEY",
	},
}

// Config represents runtime configuration.
type Config struct {
	Preset              string
	BaseURL             string
	Model               string
	APIKey              string
	Temperature         *float64
	MaxTokens           *int
	MaxCompletionTokens *int
	ReasoningEffort     string
	ThinkingBudget      *int
	Stream              bool
	ShowStats           bool
	ShowReasoning       bool
	OnlyReasoning       bool
	JSONOutput          bool
	SystemPrompt        string
	Files               []string
	ImagePaths          []string
}

// LoadEnvFiles fills missing or empty variables from local .env files and user configs.
func LoadEnvFiles() {
	// Try current directory .env
	loadEnvFile(".env")

	// Try executable directory's parent .env (e.g. /var/www/straitly/.env when binary is in bin/)
	if execPath, err := os.Executable(); err == nil {
		parentEnv := filepath.Join(filepath.Dir(filepath.Dir(execPath)), ".env")
		loadEnvFile(parentEnv)
	}

	// Try ~/.config/callm/config and ~/.config/straitly/config
	if home, err := os.UserHomeDir(); err == nil {
		loadEnvFile(filepath.Join(home, ".config", "callm", "config"))
		loadEnvFile(filepath.Join(home, ".config", "straitly", "config"))
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// support `export KEY=VAL` or `KEY=VAL`
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		// Strip quotes if present
		if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) ||
			(strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
			if len(v) >= 2 {
				v = v[1 : len(v)-1]
			}
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// ResolveAPIKey discovers the appropriate API key.
// Precedence:
// 1. Explicit direct key value via flagKey (--api-key or --key)
// 2. Explicit environment variable name via flagKeyEnv (--api-key-env)
// 3. CALLM_API_KEY
// 4. Preset-specific default key (and aliases)
// Missing provider credentials remain missing; unrelated provider keys are never used.
func ResolveAPIKey(preset string, flagKey string, flagKeyEnv string) (string, error) {
	if flagKey != "" {
		return flagKey, nil
	}
	if flagKeyEnv != "" {
		val := os.Getenv(flagKeyEnv)
		if val == "" {
			return "", fmt.Errorf("environment variable '%s' specified by --api-key-env is not set or empty", flagKeyEnv)
		}
		return val, nil
	}

	// Priority 1: Generic CALLM key if set
	if val := os.Getenv("CALLM_API_KEY"); val != "" {
		return val, nil
	}

	// Priority 2: Preset-specific key & aliases
	if p, ok := Presets[preset]; ok && p.KeyEnv != "" {
		if val := os.Getenv(p.KeyEnv); val != "" {
			return val, nil
		}
	}
	if preset == "zai" {
		if val := os.Getenv("ZHIPU_API_KEY"); val != "" {
			return val, nil
		}
	} else if preset == "qw" {
		if val := os.Getenv("QWEN_API_KEY"); val != "" {
			return val, nil
		}
	} else if preset == "ollama" {
		// Ollama runs locally and does not require an API key
		return "ollama", nil
	}

	return "", nil
}

// ResolveBaseURL applies the same endpoint precedence to every command.
func ResolveBaseURL(preset, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if value := os.Getenv("CALLM_BASE_URL"); value != "" {
		return value
	}
	if preset == "st" && os.Getenv("STRAITLY_BASE_URL") != "" {
		return os.Getenv("STRAITLY_BASE_URL")
	}
	if preset == "oa" && os.Getenv("OPENAI_BASE_URL") != "" {
		return os.Getenv("OPENAI_BASE_URL")
	}
	return Presets[preset].BaseURL
}
