package config

import (
	"bufio"
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
	"ds": {
		Name:         "DeepSeek Direct",
		BaseURL:      "https://api.deepseek.com",
		DefaultModel: "deepseek-chat",
		KeyEnv:       "DEEPSEEK_API_KEY",
	},
}

// Config represents runtime configuration.
type Config struct {
	Preset       string
	BaseURL      string
	Model        string
	APIKey       string
	Temperature  *float64
	MaxTokens    *int
	Stream       bool
	ShowStats    bool
	ShowReasoning bool
	JSONOutput   bool
	SystemPrompt string
	Files        []string
	ImagePaths   []string
}

// LoadEnvFiles loads environment variables from .env and ~/.config/straitly/config without overwriting existing vars.
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

// ResolveAPIKey discovers the appropriate API key for a given preset and explicit flag.
func ResolveAPIKey(preset string, flagKey string) string {
	if flagKey != "" {
		return flagKey
	}

	// Priority 1: Generic CALLM key if set
	if val := os.Getenv("CALLM_API_KEY"); val != "" {
		return val
	}

	// Priority 2: Preset-specific key
	if p, ok := Presets[preset]; ok {
		if val := os.Getenv(p.KeyEnv); val != "" {
			return val
		}
	}

	// Priority 3: Fallbacks
	if val := os.Getenv("STRAITLY_API_KEY"); val != "" {
		return val
	}
	if val := os.Getenv("OPENROUTER_API_KEY"); val != "" {
		return val
	}
	if val := os.Getenv("DEEPSEEK_API_KEY"); val != "" {
		return val
	}
	if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		return val
	}
	return ""
}
