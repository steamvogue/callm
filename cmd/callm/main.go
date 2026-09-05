package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

func printUsage() {
	fmt.Print(`callm — High-performance CLI for calling LLMs across Straitly, OpenRouter, DeepSeek, and custom OpenAI-compatible gateways.

Usage:
  callm [chat] [OPTIONS] ["PROMPT"...]
                                    Chat completion. Reads PROMPT from arguments, files, or stdin.
  callm models [FILTER]            List available models with context length, pricing, and modalities.
  callm info <MODEL>               Inspect full technical specs, pricing, and parameters for a model.
  callm raw <ENDPOINT> '<JSON>'    POST raw JSON body to any endpoint (e.g. /chat/completions).
  callm -h | --help                Show this help message.

Provider Presets:
  --st                             Straitly Gateway (default)
                                   URL: https://api.straitly.ai/v1 | Model: deepseek/deepseek-v4-flash-0731
  --or                             OpenRouter Gateway
                                   URL: https://openrouter.ai/api/v1 | Model: deepseek/deepseek-v4-flash-0731
  --ds                             DeepSeek Direct API
                                   URL: https://api.deepseek.com | Model: deepseek-chat
  --api, --base-url URL            Custom OpenAI-compatible base URL (e.g. Ollama, Groq, vLLM)

Options:
  -m, --model MODEL                Model ID override
  -k, --key, --api-key KEY         API key value override
      --key-env, --api-key-env ENV Custom environment variable name containing API key
  -s, --system SYSTEM              System prompt instruction
  -t, --temp TEMPERATURE           Sampling temperature (e.g. 0.7, 0.0)
  -n, --max-tokens N               Maximum tokens to generate
      --top-p P                    Top-p nucleus sampling
  -f, --file FILE                  Include contents of FILE in prompt context (can repeat)
      --image IMAGE                Attach image URL or local file path (base64 encoded, can repeat)
      --json-object                Request structured JSON object response_format
      --stream                     Force streaming response (default when stdout is terminal)
      --no-stream                  Disable streaming response
      --reasoning                  Display reasoning / chain-of-thought tokens (default in terminal)
      --no-reasoning               Hide reasoning tokens
      --only-reasoning             Only output reasoning tokens (suppress final answer)
      --stats                      Print token usage, latency, tok/s, and cost to stderr
      --json                       Output full unparsed JSON response

Environment Variables:
  CALLM_API_KEY, STRAITLY_API_KEY, OPENROUTER_API_KEY, DEEPSEEK_API_KEY, OPENAI_API_KEY
  CALLM_BASE_URL, STRAITLY_BASE_URL, OPENAI_BASE_URL
  CALLM_MODEL, STRAITLY_MODEL, OPENAI_MODEL

Examples:
  # Quick query using default model (deepseek/deepseek-v4-flash-0731):
  callm "Explain quantum entanglement in 2 sentences"

  # Use custom environment variable name for API key:
  callm --api-key-env=MY_SPECIAL_TOKEN "Summarize current news"

  # Pass explicit bearer API key:
  callm --api-key="sk-..." "Hello from custom key"

  # Switch to DeepSeek direct or OpenRouter presets:
  callm --ds "Write a binary search in Go"
  callm --or -m anthropic/claude-3.5-sonnet "Refactor this function"

  # Pipe stdin + add instruction:
  cat main.go | callm "Find concurrency race conditions"

  # Attach files and show reasoning + latency/cost stats:
  callm -f schema.sql --reasoning --stats "Generate 3 sample INSERT statements"

  # Attach image to a vision-capable model:
  callm --image chart.png "What does this diagram represent?"

  # Inspect model catalog and specs:
  callm models deepseek
  callm info deepseek/deepseek-v4-flash-0731
`)
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

	firstArg := os.Args[1]
	switch firstArg {
	case "-h", "--help", "help":
		printUsage()
		return
	case "models":
		runModels(ctx, os.Args[2:])
		return
	case "info":
		runInfo(ctx, os.Args[2:])
		return
	case "raw":
		runRaw(ctx, os.Args[2:])
		return
	case "chat":
		runChat(ctx, os.Args[2:])
		return
	default:
		// Default to chat if not a special subcommand
		runChat(ctx, os.Args[1:])
	}
}

func runModels(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("models", flag.ExitOnError)
	var stPreset, orPreset, dsPreset bool
	var customAPI, keyFlag, keyEnvFlag string

	fs.BoolVar(&stPreset, "st", false, "Use Straitly preset")
	fs.BoolVar(&orPreset, "or", false, "Use OpenRouter preset")
	fs.BoolVar(&dsPreset, "ds", false, "Use DeepSeek Direct preset")
	fs.StringVar(&customAPI, "api", "", "Custom API Base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API Base URL")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")

	fs.Usage = func() {
		fmt.Println("Usage: callm models [PRESET_OPTIONS] [FILTER]")
		fs.PrintDefaults()
	}

	_ = fs.Parse(args)
	filter := strings.Join(fs.Args(), " ")

	presetName := "st"
	if orPreset {
		presetName = "or"
	} else if dsPreset {
		presetName = "ds"
	}

	baseURL := config.Presets[presetName].BaseURL
	if customAPI != "" {
		baseURL = customAPI
	} else if envURL := os.Getenv("CALLM_BASE_URL"); envURL != "" {
		baseURL = envURL
	} else if envURL := os.Getenv("STRAITLY_BASE_URL"); envURL != "" && presetName == "st" {
		baseURL = envURL
	} else if envURL := os.Getenv("OPENAI_BASE_URL"); envURL != "" && customAPI == "" {
		baseURL = envURL
	}

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(fmt.Errorf("API key required. Export %s or pass --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
	}

	apiClient := client.NewClient(baseURL, apiKey)
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
	var stPreset, orPreset, dsPreset bool
	var customAPI, keyFlag, keyEnvFlag string

	fs.BoolVar(&stPreset, "st", false, "Use Straitly preset")
	fs.BoolVar(&orPreset, "or", false, "Use OpenRouter preset")
	fs.BoolVar(&dsPreset, "ds", false, "Use DeepSeek Direct preset")
	fs.StringVar(&customAPI, "api", "", "Custom API Base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API Base URL")
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")

	_ = fs.Parse(args)
	if len(fs.Args()) == 0 {
		die(errors.New("info requires a MODEL ID argument (e.g. callm info deepseek/deepseek-v4-flash-0731)"))
	}
	modelID := fs.Args()[0]

	presetName := "st"
	if orPreset {
		presetName = "or"
	} else if dsPreset {
		presetName = "ds"
	}

	baseURL := config.Presets[presetName].BaseURL
	if customAPI != "" {
		baseURL = customAPI
	} else if envURL := os.Getenv("CALLM_BASE_URL"); envURL != "" {
		baseURL = envURL
	}

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(fmt.Errorf("API key required. Export %s or pass --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
	}

	apiClient := client.NewClient(baseURL, apiKey)
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
	var keyFlag, keyEnvFlag, customAPI string
	fs.StringVar(&keyFlag, "k", "", "API key")
	fs.StringVar(&keyFlag, "key", "", "API key")
	fs.StringVar(&keyFlag, "api-key", "", "API key")
	fs.StringVar(&keyEnvFlag, "key-env", "", "Environment variable name containing API key")
	fs.StringVar(&keyEnvFlag, "api-key-env", "", "Environment variable name containing API key")
	fs.StringVar(&customAPI, "api", "", "Custom API base URL")
	fs.StringVar(&customAPI, "base-url", "", "Custom API base URL")

	_ = fs.Parse(args)
	rem := fs.Args()
	if len(rem) < 2 {
		die(errors.New("raw requires <ENDPOINT> and '<JSON>' arguments (e.g. callm raw /chat/completions '{\"model\": \"...\"}')"))
	}
	endpoint := rem[0]
	rawJSON := rem[1]

	apiKey, err := config.ResolveAPIKey("st", keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(errors.New("API key required. Set STRAITLY_API_KEY or use --api-key / --api-key-env"))
	}
	baseURL := "https://api.straitly.ai/v1"
	if customAPI != "" {
		baseURL = customAPI
	} else if envURL := os.Getenv("CALLM_BASE_URL"); envURL != "" {
		baseURL = envURL
	} else if envURL := os.Getenv("STRAITLY_BASE_URL"); envURL != "" {
		baseURL = envURL
	}

	apiClient := client.NewClient(baseURL, apiKey)
	respBytes, err := apiClient.RawRequest(ctx, endpoint, []byte(rawJSON))
	if err != nil {
		die(err)
	}

	var pretty bytesIndent
	if json.Unmarshal(respBytes, &pretty) == nil {
		indented, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(indented))
	} else {
		fmt.Println(string(respBytes))
	}
}

type bytesIndent interface{}

func runChat(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)

	var (
		stPreset      bool
		orPreset      bool
		dsPreset      bool
		customAPI     string
		modelFlag     string
		keyFlag       string
		keyEnvFlag    string
		systemPrompt  string
		tempVal       float64
		hasTemp       bool
		maxTokensVal  int
		hasMaxTokens  bool
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
	)

	fs.BoolVar(&stPreset, "st", false, "Use Straitly preset (default)")
	fs.BoolVar(&orPreset, "or", false, "Use OpenRouter preset")
	fs.BoolVar(&dsPreset, "ds", false, "Use DeepSeek Direct preset")
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

	fs.Func("t", "Sampling temperature", func(v string) error {
		hasTemp = true
		_, err := fmt.Sscanf(v, "%f", &tempVal)
		return err
	})
	fs.Func("temp", "Sampling temperature", func(v string) error {
		hasTemp = true
		_, err := fmt.Sscanf(v, "%f", &tempVal)
		return err
	})
	fs.Func("temperature", "Sampling temperature", func(v string) error {
		hasTemp = true
		_, err := fmt.Sscanf(v, "%f", &tempVal)
		return err
	})

	fs.Func("n", "Max tokens", func(v string) error {
		hasMaxTokens = true
		_, err := fmt.Sscanf(v, "%d", &maxTokensVal)
		return err
	})
	fs.Func("max-tokens", "Max tokens", func(v string) error {
		hasMaxTokens = true
		_, err := fmt.Sscanf(v, "%d", &maxTokensVal)
		return err
	})

	fs.Func("top-p", "Top-p", func(v string) error {
		hasTopP = true
		_, err := fmt.Sscanf(v, "%f", &topPVal)
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

	presetName := "st"
	if orPreset {
		presetName = "or"
	} else if dsPreset {
		presetName = "ds"
	}

	baseURL := config.Presets[presetName].BaseURL
	if customAPI != "" {
		baseURL = customAPI
	} else if envURL := os.Getenv("CALLM_BASE_URL"); envURL != "" {
		baseURL = envURL
	} else if envURL := os.Getenv("STRAITLY_BASE_URL"); envURL != "" && presetName == "st" {
		baseURL = envURL
	} else if envURL := os.Getenv("OPENAI_BASE_URL"); envURL != "" && customAPI == "" {
		baseURL = envURL
	}

	model := config.Presets[presetName].DefaultModel
	if modelFlag != "" {
		model = modelFlag
	} else if envModel := os.Getenv("CALLM_MODEL"); envModel != "" {
		model = envModel
	} else if envModel := os.Getenv("STRAITLY_MODEL"); envModel != "" && presetName == "st" {
		model = envModel
	} else if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" {
		model = envModel
	}

	apiKey, err := config.ResolveAPIKey(presetName, keyFlag, keyEnvFlag)
	if err != nil {
		die(err)
	}
	if apiKey == "" {
		die(fmt.Errorf("API key not found. Set %s or export OPENAI_API_KEY or use --api-key / --api-key-env", config.Presets[presetName].KeyEnv))
	}

	// Read file contents
	var fileSections []string
	for _, fp := range filesFlag {
		content, err := os.ReadFile(fp)
		if err != nil {
			die(fmt.Errorf("failed to read file '%s': %w", fp, err))
		}
		ext := filepath.Ext(fp)
		lang := strings.TrimPrefix(ext, ".")
		fileSections = append(fileSections, fmt.Sprintf("File `%s`:\n```%s\n%s\n```", fp, lang, string(content)))
	}

	// Read stdin if piped or redirected
	stdinData, _ := readStdinIfAvailable()

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
		Model:    model,
		Messages: messages,
	}
	if hasTemp {
		chatReq.Temperature = &tempVal
	}
	if hasMaxTokens {
		chatReq.MaxTokens = &maxTokensVal
	}
	if hasTopP {
		chatReq.TopP = &topPVal
	}
	if jsonObject {
		chatReq.ResponseFormat = &client.ResponseFormat{Type: "json_object"}
	}

	apiClient := client.NewClient(baseURL, apiKey)
	startTime := time.Now()

	// Determine streaming behavior:
	// Default to streaming unless --no-stream or --json is passed.
	isStreaming := true
	if noStreamFlag || jsonOutput {
		isStreaming = false
	} else if streamFlag {
		isStreaming = true
	}

	// Determine reasoning display:
	// Enabled by default in interactive terminal unless --no-reasoning is passed.
	displayReasoning := reasoningFlag
	if !noReasoning && (reasoningFlag || onlyReasoning || ui.IsTerminal(os.Stdout)) {
		displayReasoning = true
	}
	if noReasoning {
		displayReasoning = false
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
				fmt.Fprintln(os.Stderr, "\n[Request canceled]")
				return
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
		enc, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(enc))
		return
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		reasoning := choice.Message.Reasoning
		if reasoning == "" {
			reasoning = choice.Message.ReasoningContent
		}

		if displayReasoning && reasoning != "" {
			if ui.IsTerminal(os.Stderr) {
				fmt.Fprintf(os.Stderr, "\033[2m\033[36m[Thinking...]\n%s\033[0m\n\n", reasoning)
			} else {
				fmt.Fprintf(os.Stderr, "[Thinking...]\n%s\n\n", reasoning)
			}
		}

		if !onlyReasoning {
			fmt.Println(choice.Message.Content)
		}
	}

	if showStats {
		ui.PrintStats(os.Stderr, duration, resp.Usage, model)
	}
}

func encodeImageToDataURI(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	data, err := os.ReadFile(pathOrURL)
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

// readStdinIfAvailable reads from os.Stdin only if data is actually present or redirected from a file/pipe.
func readStdinIfAvailable() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	// Character device = interactive terminal, don't read
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", nil
	}
	// Regular file redirected via < file.txt
	if stat.Mode().IsRegular() {
		if stat.Size() == 0 {
			return "", nil
		}
		data, err := io.ReadAll(os.Stdin)
		return string(data), err
	}
	// Pipe: check if readable without blocking
	if !isStdinReadable() {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	return string(data), err
}
