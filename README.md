# callm (call - llm)

A blazing-fast, zero-dependency Go CLI utility for calling LLMs across multiple OpenAI-compatible gateways.

Features built-in presets for **Straitly**, **OpenRouter**, and **DeepSeek Direct API**, or any custom endpoint.

Default model: **`deepseek/deepseek-v4-flash-0731`**

---

## Highlights

- **Zero Runtime Dependencies**: Compiled into a single static binary (`callm`). Eliminates external dependencies on `curl`, `jq`, or Python runtimes.
- **Sub-3ms Startup**: Imperceptible cold start latency, ideal for Unix pipelines and automated scripts.
- **Provider Presets**:
  - `--st` *(default)*: Straitly Gateway (`https://api.straitly.ai/v1`)
  - `--or`: OpenRouter Gateway (`https://openrouter.ai/api/v1`)
  - `--ds`: DeepSeek Direct API (`https://api.deepseek.com`)
  - `--api=URL` / `--base-url=URL`: Custom OpenAI-compatible endpoint (Ollama, Groq, vLLM, etc.)
- **Real-Time SSE Streaming**: Native streaming with instant token delivery and graceful `Ctrl+C` handling.
- **DeepSeek Chain-of-Thought (Reasoning)**:
  - Real-time rendering of thinking tokens (`reasoning` / `reasoning_content`) before the main answer.
  - Granular control via `--reasoning`, `--no-reasoning`, and `--only-reasoning`.
- **Flexible Context Ingestion**:
  - Positional prompt arguments
  - Non-blocking piped stdin (`cat log.txt | callm "Extract errors"`)
  - Context file attachments (`-f schema.sql -f queries.sql`)
  - Multimodal Vision: `--image diagram.png` (auto base64 data-URI encoded)
- **Observability & Cost Transparency**:
  - `--stats` displays latency, token counts, tokens/sec, and exact cost in USD (from gateway `usage.cost`).
- **Catalog Explorer**:
  - `callm models [FILTER]`
  - `callm info <MODEL>`
- **Raw API Access**:
  - `callm raw <ENDPOINT> '<JSON>'`

---

## Quick Start

### 1. Build and Test

```bash
make help     # View available targets
make build    # Compile Go binary to bin/callm
make test     # Sanity check with deepseek/deepseek-v4-flash-0731
```

### 2. Configure API Keys

Keys can be configured in multiple ways:

1. **Environment Variables (Default per preset)**:

   ```bash
   export STRAITLY_API_KEY="your-straitly-key"      # for --st (default)
   export OPENROUTER_API_KEY="your-openrouter-key"  # for --or
   export DEEPSEEK_API_KEY="your-deepseek-key"      # for --ds
   export CALLM_API_KEY="your-callm-key"            # global callm override
   export OPENAI_API_KEY="your-openai-key"          # generic fallback
   ```

2. **Custom Environment Variable Name via `--api-key-env`**:

   ```bash
   callm --api-key-env=TEAM_SECRET_KEY "Summarize codebase"
   ```

3. **Explicit Key via `--api-key`**:

   ```bash
   callm --api-key="sk-..." "Hello from explicit key"
   ```

4. **Local `.env` or `~/.config/callm/config`**:

   ```bash
   echo "STRAITLY_API_KEY=your-key-here" > .env
   ```

---

## Usage Examples

### Quick Query (Default Model)

```bash
callm "Explain quantum entanglement in 2 sentences"
```

### Switch Provider Presets

```bash
# Direct DeepSeek API
callm --ds "Write an LRU cache in Go"

# OpenRouter with Claude 3.5 Sonnet
callm --or -m anthropic/claude-3.5-sonnet "Explain OKLCH color space"

# Local Ollama or custom gateway
callm --api=http://localhost:11434/v1 -m llama3 "Hello from local model"
```

### Piped Stdin + Instructions

```bash
cat main.go | callm "Find concurrency race conditions"
git diff | callm "Write a concise Git commit message"
```

### Attach Context Files & View Reasoning

```bash
callm -f schema.sql --reasoning --stats "Generate 3 sample INSERT statements"
```

### Control Reasoning Output

```bash
# Only show the model's chain-of-thought:
callm --only-reasoning "Which is larger: 9.11 or 9.9?"

# Suppress reasoning (final answer only):
callm --no-reasoning "What is the capital of France?"
```

### Multimodal Vision Models

```bash
callm -m "deepseek/deepseek-v4-flash-vision-exp" --image ./chart.png "Explain the trend shown in this chart"
```

### Model Discovery & Inspection

```bash
# Filter models by name/regex
callm models deepseek
callm models claude

# Inspect technical specs, context length, and pricing
callm info deepseek/deepseek-v4-flash-0731
```

---

## CLI Reference

```text
Usage:
  callm [chat] [OPTIONS] ["PROMPT"...]
  callm models [FILTER]
  callm info <MODEL>
  callm raw <ENDPOINT> '<JSON>'
  callm -h | --help

Provider Presets:
  --st                             Straitly Gateway (default: https://api.straitly.ai/v1)
  --or                             OpenRouter Gateway (https://openrouter.ai/api/v1)
  --ds                             DeepSeek Direct API (https://api.deepseek.com)
  --api, --base-url URL            Custom OpenAI-compatible endpoint URL

Chat Options:
  -m, --model MODEL                Model ID override (default: deepseek/deepseek-v4-flash-0731)
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
      --reasoning                  Display reasoning tokens (default in terminal)
      --no-reasoning               Hide reasoning tokens
      --only-reasoning             Only output reasoning tokens
      --stats                      Print token usage, latency, tok/s, and cost to stderr
      --json                       Output full unparsed JSON response
```

---

## Installation

Install system-wide to `/usr/local/bin`:

```bash
sudo make install
```

Or for user-only:

```bash
make install PREFIX=$HOME/.local
```
