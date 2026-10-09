package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"callm/internal/client"
	"callm/internal/config"
	"callm/internal/export"
	"callm/internal/pipeline"
	"callm/internal/ui"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var (
	// Version is injected at build time via -ldflags="-X main.Version=..."
	Version = "dev"
	// Commit is injected at build time via -ldflags="-X main.Commit=..."
	Commit = "none"
	// Date is injected at build time via -ldflags="-X main.Date=..."
	Date = "unknown"
)

func printVersion() {
	if _, err := fmt.Printf("callm %s (commit: %s, built at: %s)\n", Version, Commit, Date); err != nil {
		die(err)
	}
}

func printUsage() {
	_, writeErr := fmt.Printf(`callm %s — High-performance CLI for calling LLMs across Straitly, OpenRouter, OrcaRouter, DeepSeek, Anthropic, Moonshot, Kimi Code, Zhipu, Qwen, OpenAI, Groq, Poolside, and Ollama.

Usage:
  callm [chat] [OPTIONS] ["PROMPT"...]
                                    Chat completion. Reads PROMPT from arguments, files, or stdin.
  callm models [OPTIONS] [FILTER]  List, filter, or export models (--format json|zed|kilo|continue).
  callm info [OPTIONS] <MODEL>     Inspect full technical specs, pricing, and parameters for a model.
  callm raw [OPTIONS] <ENDPOINT> ['<JSON>']
                                    POST raw JSON body to any endpoint (e.g. /chat/completions).
  callm version | -v | --version   Print version, commit, and build date.
  callm -h | --help                Show this help message.

Provider Presets:
  --st                             Straitly Gateway
                                   URL: https://api.straitly.ai/v1 | Model: deepseek/deepseek-v4-flash-0731
  --or                             OpenRouter Gateway
                                   URL: https://openrouter.ai/api/v1 | Model: deepseek/deepseek-v4-flash-0731
  --orca                           OrcaRouter Gateway (ORCA_API_KEY)
                                   URL: https://api.orcarouter.ai/v1 | Model: orcarouter/auto
  --ds                             DeepSeek Direct API
                                   URL: https://api.deepseek.com | Model: deepseek-flash
  --ant, --anthropic               Anthropic Direct API (/v1/messages)
                                   URL: https://api.anthropic.com/v1 | Model: claude-sonnet-4-6
  --claude                         Claude Shortcut (selects Claude Sonnet 4.6 on active gateway)
  --ms, --moonshot                 Moonshot AI (pay-as-you-go)
                                   URL: https://api.moonshot.cn/v1 | Model: moonshot-v1-auto
  --kimi                           Kimi Code subscription (KIMI_API_KEY)
                                   URL: https://api.kimi.com/coding/v1 | Model: kimi-for-coding
  --zai, --glm                     Zhipu AI (GLM / ZAI)
                                   URL: https://open.bigmodel.cn/api/paas/v4 | Model: glm-4-flash
  --qw, --qwen                     Alibaba Cloud DashScope (Qwen)
                                   URL: https://dashscope.aliyuncs.com/compatible-mode/v1 | Model: qwen-plus
  --oa, --openai                   OpenAI Direct API
                                   URL: https://api.openai.com/v1 | Model: gpt-4o
  --groq                           Groq Ultra-Fast OSS
                                   URL: https://api.groq.com/openai/v1 | Model: llama-3.3-70b-versatile
  --pool                           Poolside (POOLSIDE_API_KEY)
                                   URL: https://inference.poolside.ai/v1 | Model: poolside/laguna-s-2.1
  --ollama                         Ollama Local Gateway
                                   URL: http://localhost:11434/v1 | Model: deepseek-r1
  --api, --base-url URL            Custom OpenAI-compatible base URL (e.g. vLLM, SGLang)

Options (chat unless stated otherwise):
  Provider, URL, key, --user-agent, --timeout and --header-timeout apply to all API commands.
  Flags must precede positional arguments; use COMMAND --help for command-specific help.

  -m, --model MODEL                Model ID override
  -k, --key, --api-key KEY         API key value override
      --key-env, --api-key-env ENV Custom environment variable name containing API key
      --user-agent TEXT           HTTP User-Agent override (empty string omits header)
  -s, --system SYSTEM              System prompt instruction
      --system-file FILE           Read system instructions verbatim (conflicts with --system)
      --prompt-file FILE           Read prompt text without a file wrapper
      --body-file FILE|-           Raw command JSON body from a file or stdin
      --max-input-bytes N          Combined wire-body limit (default 67108864; maximum 64 MiB)
  -t, --temp, --temperature T      Sampling temperature (omitted by default)
  -n, --max-tokens N               Maximum tokens to generate
      --max-completion-tokens N    Maximum completion tokens (OpenAI reasoning models, including known GPT-6 IDs)
      --effort, --reasoning-effort E   Reasoning effort (model-dependent; low, medium, high, xhigh, max, none)
      --thinking-budget N          Manual thinking budget (older Claude / OpenRouter; new Claude uses --effort)
      --top-p P                    Top-p nucleus sampling
  -f, --file FILE                  Include contents of FILE in prompt context (can repeat)
      --image IMAGE                Attach image URL or local file path (base64 encoded, can repeat)
      --json-object                Request structured JSON object response_format
      --schema FILE                Request JSON Schema output and validate locally (non-streaming)
      --validate-schema FILE       Validate locally without sending a provider schema
      --result-json                Versioned result/status envelope (non-streaming, implies --strict)
      --stream                     Force streaming response (default when stdout is terminal)
      --no-stream                  Disable streaming response
      --reasoning                  Display returned reasoning on stderr (default when stdout is terminal)
      --no-reasoning               Hide reasoning fields (literal content is preserved)
      --parse-think                Interpret inline <think> tags as reasoning (explicit opt-in)
      --only-reasoning             Only output reasoning tokens (suppress final answer)
      --stats                      Print token usage, latency, tok/s, and cost to stderr
      --json                       Output original JSON response (non-streaming; transport success)
      --strict                     Require usable output and a terminal reason (also with --json)
      --allow-empty                Permit intentionally empty output (still rejects failed completions)
      --header-timeout DURATION    Wait for response headers (inherits --timeout; 0 disables)
      --idle-timeout DURATION      Wait for streamed bytes (inherits --timeout; 0 disables)
      --stdin-timeout DURATION     Wait for piped input EOF (default 300s; 0 disables)
      --no-stdin                   Ignore stdin, even when it is a pipe
      --timeout DURATION           Total API timeout (default 300s; seconds or 5m; 0 disables)

Environment Variables:
  CALLM_API_KEY, STRAITLY_API_KEY, OPENROUTER_API_KEY, ORCA_API_KEY,
  DEEPSEEK_API_KEY, ANTHROPIC_API_KEY,
  OPENAI_API_KEY, MOONSHOT_API_KEY, KIMI_API_KEY, ZAI_API_KEY (alias ZHIPU_API_KEY),
  DASHSCOPE_API_KEY (alias QWEN_API_KEY), GROQ_API_KEY, POOLSIDE_API_KEY, OLLAMA_API_KEY (optional)
  CALLM_USER_AGENT
  CALLM_BASE_URL, STRAITLY_BASE_URL, OPENAI_BASE_URL
  CALLM_MODEL, STRAITLY_MODEL, OPENAI_MODEL
  CALLM_LOAD_DOTENV

Defaults and precedence:
  User-Agent: --user-agent > nonempty CALLM_USER_AGENT > project default:
    CallM (Call-LLM; +https://github.com/steamvogue/callm)
  --timeout defaults to 300 seconds (5m). Header/idle limits inherit that value.
  --stdin-timeout independently defaults to 300 seconds. Each limit accepts 0 to disable.
  Temperature, top-p, effort and token caps are omitted unless set, except Anthropic
  max_tokens defaults to 4096 (increased if needed for an implicit thinking cap).
  Key: explicit key > named key-env > CALLM_API_KEY > selected provider key/alias.
  URL/model: explicit flag > CALLM_* > selected provider STRAITLY_*/OPENAI_* > preset.
  Unset variables are filled from .env one directory above the executable, then
  ~/.config/callm/config (legacy ~/.config/straitly/config last). A .env in the
  current directory could redirect API keys, so it loads first only when
  CALLM_LOAD_DOTENV=1 is set in the environment or those files; 0 hides the notice.
  --claude replaces the preset model; explicit/model environment overrides still win.
  Without an explicit provider, --claude selects Anthropic if only its key is present
  among ANTHROPIC_API_KEY, STRAITLY_API_KEY and OPENROUTER_API_KEY.
  Default provider: Poolside. Without a preset flag, the first provider whose key
  is set wins, checked in order Poolside, OrcaRouter, Straitly, DeepSeek,
  OpenRouter, Kimi Code. Other configured keys require an explicit preset (e.g. --oa).
  Streaming/reasoning display default on only when stdout is a terminal.
  Text output rejects truncation, refusal, tools and empty answers; --allow-empty permits empty.
  --strict also requires a terminal reason; --json alone preserves diagnostic envelopes.
  Schema validation buffers output; external schema references are disabled.
  --result-json errors may contain partial answers; publish only exit 0 and status "ok".
  OrcaRouter: --effort sends reasoning_effort; --thinking-budget is unsupported.
  OrcaRouter --stats requests usage.cost_usd via X-OrcaRouter-Include-Cost.
  Kimi Code: --kimi uses subscription quota; --ms/--moonshot use Moonshot billing.
  Native Anthropic catalogs support table/JSON; editor exports require OpenAI-compatible endpoints.
  --stats reports only server-supplied cost; it does not estimate subscription cost.
  DeepSeek: --ds defaults to deepseek-flash, which thinks unless disabled; deepseek-chat
  and deepseek-reasoner are retired. raw can send "thinking":{"type":"disabled"}.
  Reasoning display flags do not enable model reasoning; --effort/--thinking-budget request it.
  Native Claude 4.6 and known newer models use adaptive thinking with --effort.
  New Claude models reject manual budgets/sampling; returned thinking may be omitted.
  Known GPT-6 IDs normalize token caps; sampling requires --effort none on Sol/Luna.
  Astra/6.1 Sol reject none; unknown/custom models retain low/medium/high effort.

Examples:
  # Quick query using the default provider (first configured key; Poolside otherwise):
  callm "Explain quantum entanglement in 2 sentences"

  # Quick query to Claude Sonnet 4.6 (via Straitly/OpenRouter):
  callm --claude "Refactor this Go function"

  # Direct Anthropic Claude with extended thinking:
  callm --ant --effort=high "Prove the Riemann hypothesis"

  # OrcaRouter automatic routing (uses ORCA_API_KEY):
  callm --orca --stats "Explain this error"

  # Moonshot Kimi or Alibaba Qwen:
  callm --ms "Search and summarize 2026 AI developments"
  callm --qw "Explain quantum computing fundamentals"

  # Kimi Code subscription (uses KIMI_API_KEY):
  callm --kimi -f main.go "Review this code for bugs"

  # Poolside (uses POOLSIDE_API_KEY):
  callm --pool "What are channels in Go?"

  # OpenAI o3-mini with reasoning effort:
  callm --oa -m o3-mini --effort=medium "Solve this competitive programming problem"

  # Pipe stdin + add instruction:
  cat main.go | callm "Find concurrency race conditions"

  # Attach files and show reasoning + latency/cost stats:
  callm -f schema.sql --reasoning --stats "Generate 3 sample INSERT statements"

  # Local Ollama model with inline <think> tags:
  callm --ollama --parse-think "Solve 17 * 23 step by step"

  # Save the model catalog or paste-ready editor configs:
  callm models --format=json --filter="deepseek,z.ai,qwen" > models.json
  callm models --format=zed deepseek
  callm models --format=kilo
`, Version)
	if writeErr != nil {
		die(writeErr)
	}
}

func main() {
	notice, err := config.LoadEnvFiles()
	if notice != "" {
		fmt.Fprintln(os.Stderr, "callm: "+notice)
	}
	if err != nil {
		die(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		// If stdin is piped, allow running bare `callm`
		if !ui.IsTerminal(os.Stdin) {
			runChat(ctx, []string{})
			return
		}
		printUsage()
		return
	}

	firstArg, commandArgs := splitCommand(os.Args[1:])
	switch firstArg {
	case "-v", "--version", "version":
		printVersion()
		return
	case "-h", "--help", "help":
		printUsage()
		return
	case "models":
		runModels(ctx, commandArgs)
		return
	case "info":
		runInfo(ctx, commandArgs)
		return
	case "raw":
		runRaw(ctx, commandArgs)
		return
	case "chat":
		runChat(ctx, commandArgs)
		return
	default:
		// Default to chat if not a special subcommand
		runChat(ctx, commandArgs)
	}
}

type presetFlags struct {
	stPreset   bool
	orPreset   bool
	orcaPreset bool
	dsPreset   bool
	antPreset  bool
	msPreset   bool
	kimiPreset bool
	zaiPreset  bool
	qwPreset   bool
	oaPreset   bool
	groqPreset bool
	poolPreset bool
	olPreset   bool
	claudeFlag bool
}

func (p *presetFlags) Register(fs *flag.FlagSet) {
	fs.BoolVar(&p.stPreset, "st", false, "Use Straitly preset")
	fs.BoolVar(&p.orPreset, "or", false, "Use OpenRouter preset")
	fs.BoolVar(&p.orcaPreset, "orca", false, "Use OrcaRouter preset (ORCA_API_KEY)")
	fs.BoolVar(&p.dsPreset, "ds", false, "Use DeepSeek Direct preset")
	fs.BoolVar(&p.antPreset, "ant", false, "Use Anthropic Direct API preset")
	fs.BoolVar(&p.antPreset, "anthropic", false, "Use Anthropic Direct API preset")
	fs.BoolVar(&p.claudeFlag, "claude", false, "Claude chat model shortcut; may select Anthropic when only its key is present")
	fs.BoolVar(&p.msPreset, "ms", false, "Use Moonshot AI (Kimi) preset")
	fs.BoolVar(&p.msPreset, "moonshot", false, "Use Moonshot AI (Kimi) preset")
	fs.BoolVar(&p.kimiPreset, "kimi", false, "Use Kimi Code subscription preset (KIMI_API_KEY)")
	fs.BoolVar(&p.zaiPreset, "zai", false, "Use Zhipu AI (GLM) preset")
	fs.BoolVar(&p.zaiPreset, "glm", false, "Use Zhipu AI (GLM) preset")
	fs.BoolVar(&p.qwPreset, "qw", false, "Use Alibaba DashScope (Qwen) preset")
	fs.BoolVar(&p.qwPreset, "qwen", false, "Use Alibaba DashScope (Qwen) preset")
	fs.BoolVar(&p.oaPreset, "oa", false, "Use OpenAI Direct preset")
	fs.BoolVar(&p.oaPreset, "openai", false, "Use OpenAI Direct preset")
	fs.BoolVar(&p.groqPreset, "groq", false, "Use Groq OSS preset")
	fs.BoolVar(&p.poolPreset, "pool", false, "Use Poolside preset (POOLSIDE_API_KEY)")
	fs.BoolVar(&p.olPreset, "ollama", false, "Use Ollama Local preset")
}

func (p *presetFlags) ResolvePreset() string {
	count := 0
	for _, enabled := range []bool{p.stPreset, p.orPreset, p.orcaPreset, p.dsPreset, p.antPreset, p.msPreset, p.kimiPreset, p.zaiPreset, p.qwPreset, p.oaPreset, p.groqPreset, p.poolPreset, p.olPreset} {
		if enabled {
			count++
		}
	}
	if count > 1 {
		die(errors.New("select only one provider preset"))
	}
	if p.claudeFlag && (p.dsPreset || p.msPreset || p.kimiPreset || p.zaiPreset || p.qwPreset || p.oaPreset || p.groqPreset || p.poolPreset || p.olPreset) {
		die(errors.New("--claude requires Straitly, OpenRouter, OrcaRouter, or Anthropic"))
	}

	if p.orPreset {
		return "or"
	}
	if p.orcaPreset {
		return "orca"
	}
	if p.dsPreset {
		return "ds"
	}
	if p.antPreset {
		return "ant"
	}
	if p.msPreset {
		return "ms"
	}
	if p.kimiPreset {
		return "kimi"
	}
	if p.zaiPreset {
		return "zai"
	}
	if p.qwPreset {
		return "qw"
	}
	if p.oaPreset {
		return "oa"
	}
	if p.groqPreset {
		return "groq"
	}
	if p.poolPreset {
		return "pool"
	}
	if p.olPreset {
		return "ollama"
	}
	if p.stPreset {
		return "st"
	}
	if p.claudeFlag {
		if os.Getenv("ANTHROPIC_API_KEY") != "" && os.Getenv("STRAITLY_API_KEY") == "" && os.Getenv("OPENROUTER_API_KEY") == "" {
			return "ant"
		}
	}
	return detectDefaultPreset()
}

// detectDefaultPreset returns the first provider whose key is set, checked in
// the documented priority order; otherwise Poolside, the default provider.
func detectDefaultPreset() string {
	for _, name := range []string{"pool", "orca", "st", "ds", "or", "kimi"} {
		if os.Getenv(config.Presets[name].KeyEnv) != "" {
			return name
		}
	}
	return "pool"
}

// missingKeyError lists variable names and explicit presets, never credentials.
func missingKeyError(preset string) error {
	selected := config.Presets[preset]
	message := fmt.Sprintf("API key not found for %s (--%s). Set %s or CALLM_API_KEY or use --api-key / --api-key-env", selected.Name, preset, selected.KeyEnv)
	var available []string
	for _, name := range []string{"pool", "orca", "st", "ds", "or", "kimi", "ant", "oa", "ms", "zai", "qw", "groq", "ollama"} {
		if name == preset {
			continue
		}
		key := config.Presets[name].KeyEnv
		if os.Getenv(key) == "" {
			switch name {
			case "zai":
				key = "ZHIPU_API_KEY"
			case "qw":
				key = "QWEN_API_KEY"
			}
		}
		if os.Getenv(key) != "" {
			available = append(available, fmt.Sprintf("%s: --%s", key, name))
		}
	}
	if len(available) > 0 {
		message += ". Configured keys for other presets (select one explicitly): " + strings.Join(available, ", ")
	}
	return errors.New(message)
}

// registerTimeout accepts either seconds or a duration such as "5m" or "500ms".
func registerTimeout(fs *flag.FlagSet) *time.Duration {
	return registerDuration(fs, "timeout", client.DefaultTimeout)
}

func registerDuration(fs *flag.FlagSet, name string, defaultValue time.Duration) *time.Duration {
	timeout := defaultValue
	description := fmt.Sprintf("%s (default %.0fs; seconds or duration; 0 disables)", name, defaultValue.Seconds())
	if name == "header-timeout" || name == "idle-timeout" {
		description = name + " (inherits --timeout; seconds or duration; 0 disables)"
	}
	fs.Func(name, description, func(value string) error {
		duration, err := time.ParseDuration(value)
		if err != nil {
			duration, err = time.ParseDuration(value + "s")
		}
		if err != nil || duration < 0 {
			return fmt.Errorf("timeout must be non-negative seconds or a duration such as 300s or 5m")
		}
		timeout = duration
		return nil
	})
	return &timeout
}

// An empty environment value keeps the project default; an explicit empty flag
// suppresses the header, including net/http's built-in user agent.
func registerUserAgent(fs *flag.FlagSet) *string {
	value := os.Getenv("CALLM_USER_AGENT")
	if value == "" {
		value = client.DefaultUserAgent
	}
	return fs.String("user-agent", value, "HTTP User-Agent (CALLM_USER_AGENT; empty flag omits header)")
}

func runModels(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("models", flag.ExitOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
	var pFlags presetFlags
	pFlags.Register(fs)
	var customAPI, keyFlag, keyEnvFlag string
	var formatFlag, providerName string
	var jsonFlag bool
	var filterValues stringSlice

	fs.StringVar(&formatFlag, "format", "table", "Output format: table, json, zed, kilo, continue (vscode is an alias for continue)")
	fs.BoolVar(&jsonFlag, "json", false, "Alias for --format=json")
	fs.Var(&filterValues, "filter", "Comma-separated case-insensitive substring filters (repeatable)")
	fs.StringVar(&providerName, "provider-name", "", "Provider id/section name used by zed and kilo output")
	fs.StringVar(&customAPI, "api", "", "Custom API Base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API Base URL")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: callm models [OPTIONS] [FILTER]")
		fmt.Fprintln(fs.Output(), "  FILTER is a case-insensitive regex over model IDs and canonical slugs.")
		fmt.Fprintln(fs.Output(), "  --filter terms are comma-separated normalized substrings (z.ai matches z-ai).")
		fmt.Fprintln(fs.Output(), "  Native Anthropic endpoints support table/JSON, not editor configuration exports.")
		fs.PrintDefaults()
	}

	_ = fs.Parse(args)
	filter := strings.Join(fs.Args(), " ")

	format := export.FormatTable
	if flagWasSet(fs, "format") {
		normalized, err := export.NormalizeFormat(formatFlag)
		if err != nil {
			die(err)
		}
		format = normalized
	}
	if jsonFlag {
		if format != export.FormatTable && format != export.FormatJSON {
			die(fmt.Errorf("--json conflicts with --format=%s", format))
		}
		format = export.FormatJSON
	}

	presetName := pFlags.ResolvePreset()
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(missingKeyError(presetName))
	}

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}
	if apiClient.Provider == "ant" && format != export.FormatTable && format != export.FormatJSON {
		die(fmt.Errorf("%s export does not support native Anthropic; use --format=json and configure the editor native provider", format))
	}
	models, err := apiClient.ListModels(ctx)
	if err != nil {
		die(err)
	}

	filtered, err := ui.FilterModels(models, filter, filterValues)
	if err != nil {
		die(err)
	}
	if format == export.FormatTable {
		if err := ui.PrintModelsTable(os.Stdout, filtered, ""); err != nil {
			die(err)
		}
		return
	}

	providerNameRaw := providerName
	if providerNameRaw == "" {
		providerNameRaw = defaultProviderName(presetName, customAPI, baseURL)
	}
	providerID := export.Slug(providerNameRaw)
	if providerID == "" {
		providerID = "callm"
	}
	protocol := "openai"
	if apiClient.Provider == "ant" {
		protocol = "anthropic"
	}
	meta := export.Meta{
		Protocol:     protocol,
		BaseURL:      baseURL,
		ProviderID:   providerID,
		ProviderName: providerNameRaw,
		KeyEnv:       config.Presets[presetName].KeyEnv,
	}

	data, warnings, err := export.Render(format, filtered, meta)
	if err != nil {
		die(err)
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "warning: "+warning)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		die(err)
	}
	if format == export.FormatContinue {
		fmt.Fprintf(os.Stderr, "hint: set the Continue API key (\"apiKey\") or export %s in your shell\n", meta.KeyEnv)
	}
}

// defaultProviderName prefers the selected preset name and falls back to the
// endpoint host for custom base URLs.
func defaultProviderName(presetName, customAPI, baseURL string) string {
	if customAPI == "" {
		if preset, ok := config.Presets[presetName]; ok && preset.Name != "" {
			return preset.Name
		}
	}
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return "callm"
}

func runInfo(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
	var pFlags presetFlags
	pFlags.Register(fs)
	var customAPI, keyFlag, keyEnvFlag string

	fs.StringVar(&customAPI, "api", "", "Custom API Base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API Base URL")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: callm info [OPTIONS] <MODEL>")
		fs.PrintDefaults()
	}

	_ = fs.Parse(args)
	if len(fs.Args()) == 0 {
		die(errors.New("info requires a MODEL ID argument (e.g. callm info deepseek/deepseek-v4-flash-0731)"))
	}
	modelID := fs.Args()[0]

	presetName := pFlags.ResolvePreset()
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(missingKeyError(presetName))
	}

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}
	models, err := apiClient.ListModels(ctx)
	if err != nil {
		die(err)
	}

	for _, m := range models {
		if m.ID == modelID || m.CanonicalSlug == modelID {
			if err := ui.PrintModelInfo(os.Stdout, m); err != nil {
				die(err)
			}
			return
		}
	}
	die(fmt.Errorf("model '%s' not found on %s", modelID, baseURL))
}

func runRaw(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("raw", flag.ContinueOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
	stdinTimeout := registerDuration(fs, "stdin-timeout", client.DefaultTimeout)
	maxInput := registerInputLimit(fs)
	bodyFile := fs.String("body-file", "", "JSON body from a regular file, or - for stdin")
	var keyFlag, keyEnvFlag, customAPI string
	var pFlags presetFlags
	pFlags.Register(fs)
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")
	fs.StringVar(&customAPI, "api", "", "Custom API base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API base URL")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: callm raw [OPTIONS] <ENDPOINT> ['<JSON>'] (or --body-file FILE|-)")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		die(err)
	}
	rem := fs.Args()
	if len(rem) != 2 && !(len(rem) == 1 && *bodyFile != "") {
		die(errors.New("raw requires <ENDPOINT> and either one JSON argument or --body-file FILE|-"))
	}
	endpoint := rem[0]
	var body []byte
	var err error
	if *bodyFile != "" {
		if len(rem) != 1 {
			die(errors.New("--body-file conflicts with a positional JSON body"))
		}
		if *bodyFile == "-" {
			inputCtx, cancel := inputContext(ctx, *stdinTimeout)
			data, inputErr := readStdinWithLimit(inputCtx, *maxInput)
			cancel()
			body, err = []byte(data), inputErr
		} else {
			body, err = readContextFileLimit(*bodyFile, *maxInput)
		}
	} else {
		body = []byte(rem[1])
	}
	if err != nil {
		die(fmt.Errorf("raw body: %w", err))
	}
	if !json.Valid(body) {
		die(errors.New("raw body must be one valid JSON value"))
	}

	presetName := pFlags.ResolvePreset()
	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(missingKeyError(presetName))
	}
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	apiClient.MaxRequestBytes = *maxInput
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}
	respBytes, err := apiClient.RawRequest(ctx, endpoint, body)
	if err != nil {
		die(err)
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, respBytes, "", "  ") == nil {
		if _, err := fmt.Println(pretty.String()); err != nil {
			die(err)
		}
	} else {
		if _, err := fmt.Println(string(respBytes)); err != nil {
			die(err)
		}
	}
}

func runChat(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
	maxInput := registerInputLimit(fs)
	promptFile := fs.String("prompt-file", "", "Prompt text from a regular file, without wrappers")
	systemFile := fs.String("system-file", "", "System instructions from a regular file (conflicts with --system)")
	schemaFile := fs.String("schema", "", "JSON Schema to request and validate locally")
	validateSchemaFile := fs.String("validate-schema", "", "JSON Schema for local validation only")
	resultOutput := fs.Bool("result-json", false, "Versioned result/status envelope (implies --strict)")
	idleTimeout := registerDuration(fs, "idle-timeout", client.DefaultTimeout)

	stdinTimeout := registerDuration(fs, "stdin-timeout", client.DefaultTimeout)
	var noStdin bool
	fs.BoolVar(&noStdin, "no-stdin", false, "Do not read stdin")

	var (
		pFlags        presetFlags
		customAPI     string
		modelFlag     string
		keyFlag       string
		keyEnvFlag    string
		systemPrompt  string
		effortFlag    string
		tempVal       float64
		hasTemp       bool
		maxTokensVal  int
		hasMaxTokens  bool
		maxCompTokens int
		hasMaxComp    bool
		thinkingBud   int
		hasThinking   bool
		topPVal       float64
		hasTopP       bool
		filesFlag     stringSlice
		imagesFlag    stringSlice
		jsonObject    bool
		streamFlag    bool
		noStreamFlag  bool
		reasoningFlag bool
		noReasoning   bool
		onlyReasoning bool
		parseThinking bool
		showStats     bool
		jsonOutput    bool
		strictOutput  bool
		allowEmpty    bool
		versionFlag   bool
	)

	fs.BoolVar(&versionFlag, "v", false, "Show version")
	fs.BoolVar(&versionFlag, "version", false, "Show version")
	pFlags.Register(fs)

	fs.StringVar(&customAPI, "api", "", "Custom API base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API base URL")
	fs.StringVar(&modelFlag, "m", "", "Model ID")
	fs.StringVar(&modelFlag, "model", "", "Model ID")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")
	fs.StringVar(&systemPrompt, "s", "", "System prompt")
	fs.StringVar(&systemPrompt, "system", "", "System prompt")
	fs.StringVar(&effortFlag, "effort", "", "Reasoning effort (model-dependent; low, medium, high, xhigh, max, none)")
	fs.StringVar(&effortFlag, "reasoning-effort", "", "Reasoning effort (model-dependent; low, medium, high, xhigh, max, none)")

	fs.Func("t", "Sampling temperature", func(v string) error {
		hasTemp = true
		var err error
		tempVal, err = strconv.ParseFloat(v, 64)
		return err
	})
	fs.Func("temp", "Sampling temperature", func(v string) error {
		hasTemp = true
		var err error
		tempVal, err = strconv.ParseFloat(v, 64)
		return err
	})
	fs.Func("temperature", "Sampling temperature", func(v string) error {
		hasTemp = true
		var err error
		tempVal, err = strconv.ParseFloat(v, 64)
		return err
	})

	fs.Func("n", "Max tokens", func(v string) error {
		hasMaxTokens = true
		var err error
		maxTokensVal, err = strconv.Atoi(v)
		return err
	})
	fs.Func("max-tokens", "Max tokens", func(v string) error {
		hasMaxTokens = true
		var err error
		maxTokensVal, err = strconv.Atoi(v)
		return err
	})
	fs.Func("max-completion-tokens", "Max completion tokens", func(v string) error {
		hasMaxComp = true
		var err error
		maxCompTokens, err = strconv.Atoi(v)
		return err
	})
	fs.Func("thinking-budget", "Thinking token budget", func(v string) error {
		hasThinking = true
		var err error
		thinkingBud, err = strconv.Atoi(v)
		return err
	})

	fs.Func("top-p", "Top-p", func(v string) error {
		hasTopP = true
		var err error
		topPVal, err = strconv.ParseFloat(v, 64)
		return err
	})

	fs.Var(&filesFlag, "f", "File path to include in prompt context (can repeat)")
	fs.Var(&filesFlag, "file", "File path to include in prompt context (can repeat)")
	fs.Var(&imagesFlag, "image", "Image path or URL to attach (can repeat)")

	fs.BoolVar(&jsonObject, "json-object", false, "Enforce json_object response format")
	fs.BoolVar(&streamFlag, "stream", false, "Force streaming output")
	fs.BoolVar(&noStreamFlag, "no-stream", false, "Disable streaming output")
	fs.BoolVar(&reasoningFlag, "reasoning", false, "Display reasoning tokens")
	fs.BoolVar(&noReasoning, "no-reasoning", false, "Hide reasoning tokens")
	fs.BoolVar(&onlyReasoning, "only-reasoning", false, "Only display reasoning tokens")
	fs.BoolVar(&parseThinking, "parse-think", false, "Interpret inline <think> tags as reasoning (may alter literal content)")
	fs.BoolVar(&showStats, "stats", false, "Print stats to stderr")
	fs.BoolVar(&jsonOutput, "json", false, "Output full unparsed JSON (transport success unless --strict)")
	fs.BoolVar(&strictOutput, "strict", false, "Require usable output and a terminal finish/stop reason, including with --json")
	fs.BoolVar(&allowEmpty, "allow-empty", false, "Permit intentionally empty output; still reject failed completions")

	fs.Usage = printUsage

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		die(err)
	}

	if versionFlag {
		printVersion()
		return
	}
	if *schemaFile != "" && *validateSchemaFile != "" {
		die(errors.New("choose --schema or --validate-schema"))
	}
	if *systemFile != "" && (flagWasSet(fs, "s") || flagWasSet(fs, "system")) {
		die(errors.New("--system-file conflicts with --system"))
	}
	if *resultOutput && (jsonOutput || reasoningFlag || noReasoning || onlyReasoning || parseThinking || allowEmpty) {
		die(errors.New("--result-json conflicts with --json, reasoning display controls and --allow-empty"))
	}
	var outputSchema *pipeline.Schema
	path := *schemaFile
	if path == "" {
		path = *validateSchemaFile
	}
	if path != "" {
		if jsonObject || parseThinking || onlyReasoning {
			die(errors.New("schema options conflict with --json-object, --parse-think and --only-reasoning"))
		}
		data, err := readContextFileLimit(path, pipeline.MaxSchemaBytes)
		if err != nil {
			die(fmt.Errorf("schema: %w", err))
		}
		outputSchema, err = pipeline.CompileSchema(data)
		if err != nil {
			die(err)
		}
		strictOutput = true
	}
	if *resultOutput {
		strictOutput = true
	}
	if streamFlag && (*resultOutput || outputSchema != nil) {
		die(errors.New("--stream conflicts with --result-json and schema validation"))
	}
	if *systemFile != "" {
		data, err := readContextFileLimit(*systemFile, *maxInput)
		if err != nil {
			die(fmt.Errorf("system file: %w", err))
		}
		systemPrompt = string(data)
	}
	var promptText string
	if *promptFile != "" {
		data, err := readContextFileLimit(*promptFile, *maxInput)
		if err != nil {
			die(fmt.Errorf("prompt file: %w", err))
		}
		promptText = string(data)
	}
	inputSize := len(systemPrompt) + len(promptText) + len(strings.Join(fs.Args(), " "))
	checkSize := func(added int) {
		inputSize += added
		if inputSize > *maxInput {
			die(fmt.Errorf("combined input exceeds %d bytes", *maxInput))
		}
	}
	checkSize(0)

	presetName := pFlags.ResolvePreset()
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	model := config.Presets[presetName].DefaultModel
	if pFlags.claudeFlag {
		if presetName == "ant" {
			model = "claude-sonnet-4-6"
		} else {
			model = "anthropic/claude-sonnet-4.6"
		}
	}
	if modelFlag != "" {
		model = modelFlag
	} else if envModel := os.Getenv("CALLM_MODEL"); envModel != "" {
		model = envModel
	} else if envModel := os.Getenv("STRAITLY_MODEL"); envModel != "" && presetName == "st" {
		model = envModel
	} else if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" && presetName == "oa" {
		model = envModel
	}

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(missingKeyError(presetName))
	}

	// Read file contents
	var fileSections []string
	for _, fp := range filesFlag {
		content, err := readContextFileLimit(fp, *maxInput-inputSize)
		if err != nil {
			die(fmt.Errorf("failed to read file '%s': %w", fp, err))
		}
		ext := filepath.Ext(fp)
		lang := strings.TrimPrefix(ext, ".")
		section := fmt.Sprintf("File `%s`:\n```%s\n%s\n```", fp, lang, string(content))
		checkSize(len(section) + 2)
		fileSections = append(fileSections, section)
	}

	// Read stdin if piped or redirected
	var stdinData string
	if !noStdin {
		inputCtx := ctx
		cancel := func() {}
		if *stdinTimeout > 0 {
			inputCtx, cancel = context.WithTimeout(ctx, *stdinTimeout)
		}
		stdinData, err = readStdinWithLimit(inputCtx, *maxInput-inputSize)
		cancel()
		if err != nil {
			die(fmt.Errorf("stdin: %w", err))
		}
		checkSize(len(stdinData))
	}

	// Positional arguments
	promptArgs := strings.Join(fs.Args(), " ")

	// Combine into final user prompt
	var promptBuilder strings.Builder
	if len(fileSections) > 0 {
		promptBuilder.WriteString(strings.Join(fileSections, "\n\n"))
		promptBuilder.WriteString("\n\n")
	}
	if stdinData != "" {
		promptBuilder.WriteString(stdinData)
		promptBuilder.WriteString("\n\n")
	}
	if promptArgs != "" {
		if promptText != "" {
			promptBuilder.WriteString(promptText)
			promptBuilder.WriteString("\n\n")
		}
		promptBuilder.WriteString(promptArgs)
	} else {
		promptBuilder.WriteString(promptText)
	}

	userPrompt := strings.TrimSpace(promptBuilder.String())
	if *promptFile != "" {
		userPrompt = promptBuilder.String()
	}
	if strings.TrimSpace(userPrompt) == "" && len(imagesFlag) == 0 {
		die(errors.New("no prompt provided (via arguments, -f file, or stdin)"))
	}

	// Build messages
	var messages []client.Message
	if systemPrompt != "" {
		messages = append(messages, client.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Check if multimodal
	if len(imagesFlag) > 0 {
		var parts []client.ContentPart
		if userPrompt != "" {
			parts = append(parts, client.ContentPart{
				Type: "text",
				Text: userPrompt,
			})
		}
		for _, imgPath := range imagesFlag {
			dataURI, err := encodeImageToDataURI(imgPath)
			if err != nil {
				die(fmt.Errorf("failed to encode image '%s': %w", imgPath, err))
			}
			parts = append(parts, client.ContentPart{
				Type: "image_url",
				ImageURL: &client.ImageURL{
					URL: dataURI,
				},
			})
			checkSize(len(dataURI))
		}
		messages = append(messages, client.Message{
			Role:    "user",
			Content: parts,
		})
	} else {
		messages = append(messages, client.Message{
			Role:    "user",
			Content: userPrompt,
		})
	}

	chatReq := client.ChatRequest{
		Model:           model,
		Messages:        messages,
		ReasoningEffort: effortFlag,
	}
	if hasTemp {
		chatReq.Temperature = &tempVal
	}
	if hasMaxTokens {
		chatReq.MaxTokens = &maxTokensVal
	}
	if hasMaxComp {
		chatReq.MaxCompletionTokens = &maxCompTokens
	}
	if hasThinking {
		chatReq.Thinking = &client.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: thinkingBud,
		}
	}
	if hasTopP {
		chatReq.TopP = &topPVal
	}
	if jsonObject {
		chatReq.ResponseFormat = &client.ResponseFormat{Type: "json_object"}
	}
	if *schemaFile != "" {
		chatReq.ResponseFormat = &client.ResponseFormat{Type: "json_schema", JSONSchema: &client.JSONSchemaFormat{Name: "callm_output", Strict: true, Schema: outputSchema.Raw}}
	}

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	apiClient.MaxRequestBytes = *maxInput
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if !flagWasSet(fs, "idle-timeout") {
		*idleTimeout = *timeout
	}
	apiClient.IncludeCost = showStats
	apiClient.StreamIdleTimeout = *idleTimeout
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}
	startTime := time.Now()

	// Conflicting controls are rejected instead of silently overriding each other.
	streamSeen := false
	reasoningSeen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "stream" {
			streamSeen = true
		}
		if f.Name == "reasoning" {
			reasoningSeen = true
		}
	})
	if streamFlag && (noStreamFlag || jsonOutput) {
		die(errors.New("--stream conflicts with --no-stream and --json"))
	}
	if (reasoningFlag || onlyReasoning) && noReasoning {
		die(errors.New("--no-reasoning conflicts with --reasoning and --only-reasoning"))
	}
	if jsonOutput && (reasoningFlag || noReasoning || onlyReasoning || parseThinking) {
		die(errors.New("reasoning display controls cannot filter full --json output"))
	}
	isStreaming := ui.IsTerminal(os.Stdout)
	if streamSeen {
		isStreaming = streamFlag
	}
	if noStreamFlag || jsonOutput || *resultOutput || outputSchema != nil {
		isStreaming = false
	}
	displayReasoning := ui.IsTerminal(os.Stdout)
	if reasoningSeen {
		displayReasoning = reasoningFlag
	}
	if onlyReasoning {
		displayReasoning = true
	}
	if noReasoning {
		displayReasoning = false
	}
	if isStreaming && showStats {
		chatReq.StreamOptions = &client.StreamOptions{IncludeUsage: true}
	}

	if isStreaming {
		var completion client.Completion
		renderer := ui.NewStreamRenderer(os.Stdout, os.Stderr, displayReasoning, onlyReasoning)
		renderer.ParseThinking = parseThinking
		usage, err := apiClient.StreamChat(ctx, chatReq, func(chunk client.StreamChunk) error {
			if len(chunk.Choices) > 0 {
				completion.Add(chunk.Choices[0])
				return renderer.HandleDelta(chunk.Choices[0].Delta)
			}
			return nil
		})
		err = errors.Join(err, renderer.Finish())

		completion.HasContent = renderer.HasContent
		completion.HasReasoning = renderer.HasReasoned
		if err == nil {
			err = completion.Validate(strictOutput, allowEmpty, onlyReasoning)
		}
		if showStats {
			err = errors.Join(err, ui.PrintStats(os.Stderr, time.Since(startTime), usage, model))
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				die(fmt.Errorf("request canceled: %w", err))
			}
			die(err)
		}

		return
	}

	// Non-streaming completion
	resp, err := apiClient.Chat(ctx, chatReq)
	duration := time.Since(startTime)
	if *resultOutput {
		result := pipeline.NewResult(model, resp, duration, outputSchema, err)
		if writeErr := json.NewEncoder(os.Stdout).Encode(result); writeErr != nil {
			die(writeErr)
		}
		if result.Error != nil {
			die(errors.New(result.Error.Message))
		}
		if showStats {
			if err := ui.PrintStats(os.Stderr, duration, resp.Usage, model); err != nil {
				die(err)
			}
		}
		return
	}
	if err != nil {
		die(err)
	}
	if outputSchema != nil {
		if err := resp.Choices[0].Completion().Validate(true, allowEmpty, false); err != nil {
			die(err)
		}
		if err := outputSchema.Validate(resp.Choices[0].Message.Content); err != nil {
			die(err)
		}
	}

	if showStats {
		if err := ui.PrintStats(os.Stderr, duration, resp.Usage, model); err != nil {
			die(err)
		}
	}

	if jsonOutput {
		if strictOutput {
			if err := resp.Choices[0].Completion().Validate(true, allowEmpty, false); err != nil {
				die(err)
			}
		}
		if _, err := fmt.Println(string(resp.Raw)); err != nil {
			die(err)
		}
		return
	}
	message := resp.Choices[0].Message
	var answer, reasoning bytes.Buffer
	renderer := ui.NewStreamRenderer(&answer, &reasoning, displayReasoning, onlyReasoning)
	renderer.IsTTY = ui.IsTerminal(os.Stdout)
	renderer.ParseThinking = parseThinking
	renderErr := renderer.HandleDelta(client.StreamDelta{Content: message.Content, Reasoning: message.Reasoning, ReasoningContent: message.ReasoningContent, Thought: message.Thought})
	if err := errors.Join(renderErr, renderer.Finish()); err != nil {
		die(err)
	}
	completion := resp.Choices[0].Completion()
	completion.HasContent = renderer.HasContent
	completion.HasReasoning = renderer.HasReasoned
	if err := completion.Validate(strictOutput, allowEmpty, onlyReasoning); err != nil {
		die(err)
	}
	if _, err := os.Stderr.Write(reasoning.Bytes()); err != nil {
		die(err)
	}
	if _, err := os.Stdout.Write(answer.Bytes()); err != nil {
		die(err)
	}

}

func encodeImageToDataURI(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	data, err := readContextFile(pathOrURL)
	if err != nil {
		return "", err
	}
	mimeType := "image/png"
	lower := strings.ToLower(pathOrURL)
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		mimeType = "image/jpeg"
	} else if strings.HasSuffix(lower, ".webp") {
		mimeType = "image/webp"
	} else if strings.HasSuffix(lower, ".gif") {
		mimeType = "image/gif"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "callm: %v\n", err)
	os.Exit(1)
}

// readStdinIfAvailable waits for pipe EOF on all supported platforms, bounded by ctx.
func readStdinIfAvailable(ctx context.Context) (string, error) {
	return readStdinWithLimit(ctx, client.MaxRequestBytes)
}

func readStdinWithLimit(ctx context.Context, limit int) (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	stdin := os.Stdin
	go func() {
		data, err := io.ReadAll(io.LimitReader(stdin, int64(limit)+1))
		if len(data) > limit {
			err = fmt.Errorf("stdin exceeds %d bytes remaining input limit", limit)
		}
		done <- result{data, err}
	}()
	select {
	case r := <-done:
		return string(r.data), r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// splitCommand recognizes subcommands after options without scanning inside flag values.
func splitCommand(args []string) (string, []string) {
	bools := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	var presets presetFlags
	presets.Register(bools)
	for _, name := range []string{"stream", "no-stream", "reasoning", "no-reasoning", "only-reasoning", "parse-think", "json", "result-json", "stats", "json-object", "strict", "allow-empty", "no-stdin", "v", "version", "h", "help"} {
		bools.Bool(name, false, "")
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return "chat", args
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			if !hasValue && bools.Lookup(name) == nil {
				i++
			}
			continue
		}
		switch arg {
		case "models", "info", "raw", "chat":
			rest := append([]string{}, args[:i]...)
			return arg, append(rest, args[i+1:]...)
		case "version", "help":
			if i == 0 {
				return arg, args[1:]
			}
		}
		break
	}
	return "chat", args
}

func (p *presetFlags) clientProvider(preset, baseURL, explicitURL string) string {
	if preset == "st" && !p.stPreset && (explicitURL != "" || os.Getenv("CALLM_BASE_URL") != "") && baseURL != config.Presets["st"].BaseURL {
		return ""
	}
	return preset
}

// readContextFile bounds local attachments and excludes special files that can block.
func readContextFile(path string) ([]byte, error) {
	return readContextFileLimit(path, client.MaxRequestBytes)
}

func readContextFileLimit(path string, limit int) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("attachment must be a regular file")
	}
	if info.Size() > int64(limit) {
		return nil, fmt.Errorf("attachment exceeds %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if len(data) > limit {
		return nil, fmt.Errorf("attachment exceeds %d bytes", limit)
	}
	return data, err
}

func registerInputLimit(fs *flag.FlagSet) *int {
	limit := client.MaxRequestBytes
	fs.Func("max-input-bytes", "Combined wire-body limit in bytes (default 67108864; maximum 64 MiB)", func(value string) error {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > client.MaxRequestBytes {
			return errors.New("max-input-bytes must be between 1 and 67108864")
		}
		limit = parsed
		return nil
	})
	return &limit
}

func inputContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
