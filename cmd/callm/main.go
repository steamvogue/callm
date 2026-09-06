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
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"callm/internal/client"
	"callm/internal/config"
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
	fmt.Printf("callm %s (commit: %s, built at: %s)\n", Version, Commit, Date)
}

func printUsage() {
	fmt.Printf(`callm %s — High-performance CLI for calling LLMs across Straitly, OpenRouter, OrcaRouter, DeepSeek, Anthropic, Moonshot, Zhipu, Qwen, OpenAI, Groq, and Ollama.

Usage:
  callm [chat] [OPTIONS] ["PROMPT"...]
                                    Chat completion. Reads PROMPT from arguments, files, or stdin.
  callm models [OPTIONS] [FILTER]  List available models with context length, pricing, and modalities.
  callm info [OPTIONS] <MODEL>     Inspect full technical specs, pricing, and parameters for a model.
  callm raw [OPTIONS] <ENDPOINT> '<JSON>'
                                    POST raw JSON body to any endpoint (e.g. /chat/completions).
  callm version | -v | --version   Print version, commit, and build date.
  callm -h | --help                Show this help message.

Provider Presets:
  --st                             Straitly Gateway (default)
                                   URL: https://api.straitly.ai/v1 | Model: deepseek/deepseek-v4-flash-0731
  --or                             OpenRouter Gateway
                                   URL: https://openrouter.ai/api/v1 | Model: deepseek/deepseek-v4-flash-0731
  --orca                           OrcaRouter Gateway (ORCA_API_KEY)
                                   URL: https://api.orcarouter.ai/v1 | Model: orcarouter/auto
  --ds                             DeepSeek Direct API
                                   URL: https://api.deepseek.com | Model: deepseek-chat
  --ant, --anthropic               Anthropic Direct API (/v1/messages)
                                   URL: https://api.anthropic.com/v1 | Model: claude-sonnet-4-6
  --claude                         Claude Shortcut (selects Claude Sonnet 4.6 on active gateway)
  --ms, --moonshot, --kimi         Moonshot AI (Kimi)
                                   URL: https://api.moonshot.cn/v1 | Model: moonshot-v1-auto
  --zai, --glm                     Zhipu AI (GLM / ZAI)
                                   URL: https://open.bigmodel.cn/api/paas/v4 | Model: glm-4-flash
  --qw, --qwen                     Alibaba Cloud DashScope (Qwen)
                                   URL: https://dashscope.aliyuncs.com/compatible-mode/v1 | Model: qwen-plus
  --oa, --openai                   OpenAI Direct API
                                   URL: https://api.openai.com/v1 | Model: gpt-4o
  --groq                           Groq Ultra-Fast OSS
                                   URL: https://api.groq.com/openai/v1 | Model: llama-3.3-70b-versatile
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
  -t, --temp, --temperature T      Sampling temperature (omitted by default)
  -n, --max-tokens N               Maximum tokens to generate
      --max-completion-tokens N    Maximum completion tokens (for OpenAI o1/o3/o4 reasoning models)
      --effort, --reasoning-effort E   Reasoning effort: low, medium, high (omitted by default)
      --thinking-budget N          Extended thinking token budget (Claude / OpenRouter)
      --top-p P                    Top-p nucleus sampling
  -f, --file FILE                  Include contents of FILE in prompt context (can repeat)
      --image IMAGE                Attach image URL or local file path (base64 encoded, can repeat)
      --json-object                Request structured JSON object response_format
      --stream                     Force streaming response (default when stdout is terminal)
      --no-stream                  Disable streaming response
      --reasoning                  Display returned reasoning on stderr (default when stdout is terminal)
      --no-reasoning               Hide reasoning tokens
      --only-reasoning             Only output reasoning tokens (suppress final answer)
      --stats                      Print token usage, latency, tok/s, and cost to stderr
      --json                       Output original JSON response (non-streaming)
      --header-timeout DURATION    Wait for response headers (inherits --timeout; 0 disables)
      --idle-timeout DURATION      Wait for streamed bytes (inherits --timeout; 0 disables)
      --stdin-timeout DURATION     Wait for piped input EOF (default 300s; 0 disables)
      --no-stdin                   Ignore stdin, even when it is a pipe
      --timeout DURATION           Total API timeout (default 300s; seconds or 5m; 0 disables)

Environment Variables:
  CALLM_API_KEY, STRAITLY_API_KEY, OPENROUTER_API_KEY, ORCA_API_KEY,
  DEEPSEEK_API_KEY, ANTHROPIC_API_KEY,
  OPENAI_API_KEY, MOONSHOT_API_KEY, ZAI_API_KEY (alias ZHIPU_API_KEY),
  DASHSCOPE_API_KEY (alias QWEN_API_KEY), GROQ_API_KEY, OLLAMA_API_KEY (optional)
  CALLM_USER_AGENT
  CALLM_BASE_URL, STRAITLY_BASE_URL, OPENAI_BASE_URL
  CALLM_MODEL, STRAITLY_MODEL, OPENAI_MODEL

Defaults and precedence:
  User-Agent: --user-agent > nonempty CALLM_USER_AGENT > project default:
    CallM (Call-LLM; +https://github.com/steamvogue/callm)
  --timeout defaults to 300 seconds (5m). Header/idle limits inherit that value.
  --stdin-timeout independently defaults to 300 seconds. Each limit accepts 0 to disable.
  Temperature, top-p, effort and token caps are omitted unless set, except Anthropic
  max_tokens defaults to 4096 (increased if needed for an implicit thinking cap).
  Key: explicit key > named key-env > CALLM_API_KEY > selected provider key/alias.
  URL/model: explicit flag > CALLM_* > selected provider STRAITLY_*/OPENAI_* > preset.
  --claude replaces the preset model; explicit/model environment overrides still win.
  Without an explicit provider, --claude selects Anthropic if only its key is present
  among ANTHROPIC_API_KEY, STRAITLY_API_KEY and OPENROUTER_API_KEY.
  Streaming/reasoning display default on only when stdout is a terminal.
  OrcaRouter: --effort sends reasoning_effort; --thinking-budget is unsupported.
  OrcaRouter --stats requests usage.cost_usd via X-OrcaRouter-Include-Cost.
  Reasoning display flags do not enable model reasoning; --effort/--thinking-budget request it.

Examples:
  # Quick query using default model (deepseek/deepseek-v4-flash-0731):
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

  # OpenAI o3-mini with reasoning effort:
  callm --oa -m o3-mini --effort=medium "Solve this competitive programming problem"

  # Pipe stdin + add instruction:
  cat main.go | callm "Find concurrency race conditions"

  # Attach files and show reasoning + latency/cost stats:
  callm -f schema.sql --reasoning --stats "Generate 3 sample INSERT statements"

  # Local Ollama model with inline <think> tags:
  callm --ollama "Solve 17 * 23 step by step"
`, Version)
}

func main() {
	config.LoadEnvFiles()

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
	zaiPreset  bool
	qwPreset   bool
	oaPreset   bool
	groqPreset bool
	olPreset   bool
	claudeFlag bool
}

func (p *presetFlags) Register(fs *flag.FlagSet) {
	fs.BoolVar(&p.stPreset, "st", false, "Use Straitly preset (default)")
	fs.BoolVar(&p.orPreset, "or", false, "Use OpenRouter preset")
	fs.BoolVar(&p.orcaPreset, "orca", false, "Use OrcaRouter preset (ORCA_API_KEY)")
	fs.BoolVar(&p.dsPreset, "ds", false, "Use DeepSeek Direct preset")
	fs.BoolVar(&p.antPreset, "ant", false, "Use Anthropic Direct API preset")
	fs.BoolVar(&p.antPreset, "anthropic", false, "Use Anthropic Direct API preset")
	fs.BoolVar(&p.claudeFlag, "claude", false, "Claude chat model shortcut; may select Anthropic when only its key is present")
	fs.BoolVar(&p.msPreset, "ms", false, "Use Moonshot AI (Kimi) preset")
	fs.BoolVar(&p.msPreset, "moonshot", false, "Use Moonshot AI (Kimi) preset")
	fs.BoolVar(&p.msPreset, "kimi", false, "Use Moonshot AI (Kimi) preset")
	fs.BoolVar(&p.zaiPreset, "zai", false, "Use Zhipu AI (GLM) preset")
	fs.BoolVar(&p.zaiPreset, "glm", false, "Use Zhipu AI (GLM) preset")
	fs.BoolVar(&p.qwPreset, "qw", false, "Use Alibaba DashScope (Qwen) preset")
	fs.BoolVar(&p.qwPreset, "qwen", false, "Use Alibaba DashScope (Qwen) preset")
	fs.BoolVar(&p.oaPreset, "oa", false, "Use OpenAI Direct preset")
	fs.BoolVar(&p.oaPreset, "openai", false, "Use OpenAI Direct preset")
	fs.BoolVar(&p.groqPreset, "groq", false, "Use Groq OSS preset")
	fs.BoolVar(&p.olPreset, "ollama", false, "Use Ollama Local preset")
}

func (p *presetFlags) ResolvePreset() string {
	count := 0
	for _, enabled := range []bool{p.stPreset, p.orPreset, p.orcaPreset, p.dsPreset, p.antPreset, p.msPreset, p.zaiPreset, p.qwPreset, p.oaPreset, p.groqPreset, p.olPreset} {
		if enabled {
			count++
		}
	}
	if count > 1 {
		die(errors.New("select only one provider preset"))
	}
	if p.claudeFlag && (p.dsPreset || p.msPreset || p.zaiPreset || p.qwPreset || p.oaPreset || p.groqPreset || p.olPreset) {
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
	return "st"
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

	fs.StringVar(&customAPI, "api", "", "Custom API Base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API Base URL")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: callm models [OPTIONS] [FILTER]")
		fs.PrintDefaults()
	}

	_ = fs.Parse(args)
	filter := strings.Join(fs.Args(), " ")

	presetName := pFlags.ResolvePreset()
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(fmt.Errorf("API key required. Export %s or pass --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
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

	if err := ui.PrintModelsTable(os.Stdout, models, filter); err != nil {
		die(err)
	}
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
		die(fmt.Errorf("API key required. Export %s or pass --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
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
			ui.PrintModelInfo(os.Stdout, m)
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
		fmt.Fprintln(fs.Output(), "Usage: callm raw [OPTIONS] <ENDPOINT> '<JSON>'")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		die(err)
	}
	rem := fs.Args()
	if len(rem) < 2 {
		die(errors.New("raw requires <ENDPOINT> and '<JSON>' arguments (e.g. callm raw /chat/completions '{\"model\": \"...\"}')"))
	}
	endpoint := rem[0]
	rawJSON := rem[1]

	presetName := pFlags.ResolvePreset()
	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(fmt.Errorf("API key required. Export %s or CALLM_API_KEY or pass --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
	}
	baseURL := config.ResolveBaseURL(presetName, customAPI)

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
	apiClient.HTTPClient.Timeout = *timeout
	if !flagWasSet(fs, "header-timeout") {
		*headerTimeout = *timeout
	}
	if transport, ok := apiClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = *headerTimeout
	}
	respBytes, err := apiClient.RawRequest(ctx, endpoint, []byte(rawJSON))
	if err != nil {
		die(err)
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, respBytes, "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(respBytes))
	}
}

func runChat(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	timeout := registerTimeout(fs)
	headerTimeout := registerDuration(fs, "header-timeout", client.DefaultTimeout)
	userAgent := registerUserAgent(fs)
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
		showStats     bool
		jsonOutput    bool
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
	fs.StringVar(&effortFlag, "effort", "", "Reasoning effort (low, medium, high)")
	fs.StringVar(&effortFlag, "reasoning-effort", "", "Reasoning effort (low, medium, high)")

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
	fs.BoolVar(&showStats, "stats", false, "Print stats to stderr")
	fs.BoolVar(&jsonOutput, "json", false, "Output full unparsed JSON")

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
		die(fmt.Errorf("API key not found. Set %s or CALLM_API_KEY or use --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
	}

	// Read file contents
	var fileSections []string
	for _, fp := range filesFlag {
		content, err := readContextFile(fp)
		if err != nil {
			die(fmt.Errorf("failed to read file '%s': %w", fp, err))
		}
		ext := filepath.Ext(fp)
		lang := strings.TrimPrefix(ext, ".")
		fileSections = append(fileSections, fmt.Sprintf("File `%s`:\n```%s\n%s\n```", fp, lang, string(content)))
	}

	// Read stdin if piped or redirected
	var stdinData string
	if !noStdin {
		inputCtx := ctx
		cancel := func() {}
		if *stdinTimeout > 0 {
			inputCtx, cancel = context.WithTimeout(ctx, *stdinTimeout)
		}
		stdinData, err = readStdinIfAvailable(inputCtx)
		cancel()
		if err != nil {
			die(fmt.Errorf("stdin: %w", err))
		}
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
		promptBuilder.WriteString(promptArgs)
	}

	userPrompt := strings.TrimSpace(promptBuilder.String())
	if userPrompt == "" && len(imagesFlag) == 0 {
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

	apiClient := client.NewClient(baseURL, apiKey, pFlags.clientProvider(presetName, baseURL, customAPI))
	apiClient.UserAgent = *userAgent
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
	if jsonOutput && (reasoningFlag || noReasoning || onlyReasoning) {
		die(errors.New("reasoning display controls cannot filter full --json output"))
	}
	isStreaming := ui.IsTerminal(os.Stdout)
	if streamSeen {
		isStreaming = streamFlag
	}
	if noStreamFlag || jsonOutput {
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
		renderer := ui.NewStreamRenderer(os.Stdout, os.Stderr, displayReasoning, onlyReasoning)
		usage, err := apiClient.StreamChat(ctx, chatReq, func(chunk client.StreamChunk) error {
			if len(chunk.Choices) > 0 {
				renderer.HandleDelta(chunk.Choices[0].Delta)
			}
			return nil
		})
		renderer.Finish()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				die(fmt.Errorf("request canceled: %w", err))
			}
			die(err)
		}

		if showStats {
			duration := time.Since(startTime)
			ui.PrintStats(os.Stderr, duration, usage, model)
		}
		return
	}

	// Non-streaming completion
	resp, err := apiClient.Chat(ctx, chatReq)
	duration := time.Since(startTime)
	if err != nil {
		die(err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Raw))
	} else if len(resp.Choices) > 0 {
		message := resp.Choices[0].Message
		renderer := ui.NewStreamRenderer(os.Stdout, os.Stderr, displayReasoning, onlyReasoning)
		renderer.HandleDelta(client.StreamDelta{Content: message.Content, Reasoning: message.Reasoning, ReasoningContent: message.ReasoningContent, Thought: message.Thought})
		renderer.Finish()
	}

	if showStats {
		ui.PrintStats(os.Stderr, duration, resp.Usage, model)
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
		data, err := io.ReadAll(io.LimitReader(stdin, (64<<20)+1))
		if len(data) > 64<<20 {
			err = errors.New("stdin exceeds 64 MiB")
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
	for _, name := range []string{"stream", "no-stream", "reasoning", "no-reasoning", "only-reasoning", "json", "stats", "json-object", "no-stdin", "v", "version", "h", "help"} {
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
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("attachment must be a regular file")
	}
	if info.Size() > 64<<20 {
		return nil, fmt.Errorf("attachment exceeds 64 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	if len(data) > 64<<20 {
		return nil, fmt.Errorf("attachment exceeds 64 MiB")
	}
	return data, err
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
