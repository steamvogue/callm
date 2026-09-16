package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		DefaultModel: "deepseek-flash",
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
	"pool": {
		Name:         "Poolside",
		BaseURL:      "https://inference.poolside.ai/v1",
		DefaultModel: "poolside/laguna-s-2.1",
		KeyEnv:       "POOLSIDE_API_KEY",
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

// dotenvVar opts in to loading .env from the current directory.
const dotenvVar = "CALLM_LOAD_DOTENV"

const dotenvNotice = "ignoring ./.env (it could redirect API keys); " + dotenvVar + "=1 loads it, " + dotenvVar + "=0 hides this notice"

// LoadEnvFiles fills missing or empty variables from trusted files: the .env one
// directory above the executable's directory, ~/.config/callm/config, and legacy
// ~/.config/straitly/config. A current-directory .env may belong to an untrusted
// checkout that redirects credentials, so it loads first only when CALLM_LOAD_DOTENV
// is true in the environment or a trusted file. The returned notice is for stderr.
func LoadEnvFiles() (string, error) {
	var trusted []string
	// Executable directory's parent .env (e.g. /var/www/straitly/.env when binary is in bin/)
	if execPath, err := os.Executable(); err == nil {
		trusted = append(trusted, filepath.Join(filepath.Dir(filepath.Dir(execPath)), ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		trusted = append(trusted, filepath.Join(home, ".config", "callm", "config"), filepath.Join(home, ".config", "straitly", "config"))
	}
	return loadEnvFiles(".env", trusted)
}

type envEntry struct{ key, value string }

func loadEnvFiles(workspacePath string, trustedPaths []string) (string, error) {
	trusted := make([][]envEntry, len(trustedPaths))
	for i, path := range trustedPaths {
		trusted[i] = readEnvFile(path)
	}
	// The workspace file never takes part in its own opt-in decision.
	setting := os.Getenv(dotenvVar)
	for _, entries := range trusted {
		for _, entry := range entries {
			if setting == "" && entry.key == dotenvVar {
				setting = entry.value
			}
		}
	}
	load := false
	if setting != "" {
		var err error
		if load, err = strconv.ParseBool(setting); err != nil {
			return "", fmt.Errorf("%s must be 1, true, 0, or false; got %q", dotenvVar, setting)
		}
	}
	var workspace []envEntry
	if setting == "" || load {
		workspace = readEnvFile(workspacePath)
	}
	if load {
		applyEnv(workspace)
	}
	for _, entries := range trusted {
		applyEnv(entries)
	}
	if setting == "" {
		for _, entry := range workspace {
			if entry.value != "" && os.Getenv(entry.key) == "" && isCallmVariable(entry.key) {
				return dotenvNotice, nil
			}
		}
	}
	return "", nil
}

// readEnvFile parses KEY=value lines from a regular file; special files such as
// named pipes are skipped because opening them can block startup.
func readEnvFile(path string) []envEntry {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var entries []envEntry
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
		entries = append(entries, envEntry{k, v})
	}
	return entries
}

// applyEnv fills only missing or empty variables, so earlier sources keep precedence.
func applyEnv(entries []envEntry) {
	for _, entry := range entries {
		if os.Getenv(entry.key) == "" {
			_ = os.Setenv(entry.key, entry.value)
		}
	}
}

// isCallmVariable reports names that select callm credentials, endpoints, or models.
func isCallmVariable(key string) bool {
	switch key {
	case "ZHIPU_API_KEY", "QWEN_API_KEY", "STRAITLY_BASE_URL", "STRAITLY_MODEL", "OPENAI_BASE_URL", "OPENAI_MODEL":
		return true
	}
	for _, preset := range Presets {
		if preset.KeyEnv == key {
			return true
		}
	}
	return strings.HasPrefix(key, "CALLM_")
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
