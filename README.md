# callm (call - llm)

[![CI](https://github.com/steamvogue/callm/actions/workflows/ci.yml/badge.svg)](https://github.com/steamvogue/callm/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/steamvogue/callm)](https://github.com/steamvogue/callm/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A blazing-fast, zero-dependency Go CLI utility for calling LLMs across multiple OpenAI-compatible gateways.

Includes eleven provider presets, native Anthropic support, and custom OpenAI-compatible endpoints.

Default model: **`deepseek/deepseek-v4-flash-0731`**

See [release changes](CHANGELOG.md) and [agent usage instructions](skills/callm/SKILL.md).

---

## Highlights

- **Zero Runtime Dependencies**: Compiled into a single static binary (`callm`). Eliminates external dependencies on `curl`, `jq`, or Python runtimes.
- **Millisecond Startup**: Low process startup overhead for Unix pipelines and automated scripts.
- **Provider Presets**:
  - `--st` *(default)*: Straitly Gateway (`https://api.straitly.ai/v1`)
  - `--or`: OpenRouter Gateway (`https://openrouter.ai/api/v1`)
  - `--orca`: OrcaRouter Gateway (`https://api.orcarouter.ai/v1`)
  - `--ds`: DeepSeek Direct API (`https://api.deepseek.com`)
  - `--ant`, `--anthropic`: Anthropic Direct API (`https://api.anthropic.com/v1/messages`)
  - `--claude`: Claude Sonnet 4.6 shortcut across gateways
  - `--ms`, `--moonshot`, `--kimi`: Moonshot AI Kimi (`https://api.moonshot.cn/v1`)
  - `--zai`, `--glm`: Zhipu AI GLM / ZAI (`https://open.bigmodel.cn/api/paas/v4`)
  - `--qw`, `--qwen`: Alibaba Cloud DashScope Qwen (`https://dashscope.aliyuncs.com/compatible-mode/v1`)
  - `--oa`, `--openai`: OpenAI Direct API (`https://api.openai.com/v1`)
  - `--groq`: Groq Ultra-Fast OSS (`https://api.groq.com/openai/v1`)
  - `--ollama`: Ollama Local Gateway (`http://localhost:11434/v1`)
  - `--api=URL` / `--base-url=URL`: Custom OpenAI-compatible endpoint (vLLM, SGLang, etc.)
- **Real-Time SSE Streaming**: Native streaming with instant token delivery and graceful `Ctrl+C` handling.
- **Universal Chain-of-Thought (Reasoning)**:
  - Render provider-returned reasoning fields and inline thinking tags. Availability depends on the provider and model; OpenAI o-series does not expose private reasoning text.
  - **Inline `<think>` Stream Parsing**: Automatically extracts and styles reasoning tokens from open-source models (Ollama, vLLM) that stream thinking tags inline.
  - Granular control via `--reasoning`, `--no-reasoning`, and `--only-reasoning`.
  - Configurable reasoning effort: `--effort=low|medium|high` and `--thinking-budget=N`.
- **Flexible Context Ingestion**:
  - Positional prompt arguments
  - Piped stdin with cancellation and an EOF deadline (`cat log.txt | callm "Extract errors"`)
  - Context file attachments (`-f schema.sql -f queries.sql`)
  - Multimodal Vision: `--image diagram.png` (auto base64 data-URI encoded)
- **Observability & Cost Transparency**:
  - `--stats` displays latency, token counts, tokens/sec, and reported cost in USD when supplied by the gateway (`usage.cost`, or OrcaRouter’s `usage.cost_usd`).
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
make test     # Run local tests; no provider calls
```

### 2. Configure API Keys

Keys can be configured in multiple ways:

1. **Environment Variables (Default per preset)**:

   ```bash
   export STRAITLY_API_KEY="your-straitly-key"      # for --st (default)
   export OPENROUTER_API_KEY="your-openrouter-key"  # for --or
   export ORCA_API_KEY="your-orcarouter-key"       # for --orca
   export DEEPSEEK_API_KEY="your-deepseek-key"      # for --ds
   export CALLM_API_KEY="your-callm-key"            # global callm override
   export OPENAI_API_KEY="your-openai-key"          # for --oa
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

### Configuration precedence and provider defaults

Keys resolve in this order: `--api-key`/`--key`/`-k`, the variable named by
`--api-key-env`/`--key-env`, `CALLM_API_KEY`, then the selected provider's key below.
An explicitly named but missing key variable is an error. Unrelated provider keys
are never used as fallbacks. Ollama can run without a key.

| Preset | Default chat model | Key environment variable |
| --- | --- | --- |
| `--st` (default) | `deepseek/deepseek-v4-flash-0731` | `STRAITLY_API_KEY` |
| `--or` | `deepseek/deepseek-v4-flash-0731` | `OPENROUTER_API_KEY` |
| `--orca` | `orcarouter/auto` | `ORCA_API_KEY` |
| `--ds` | `deepseek-chat` | `DEEPSEEK_API_KEY` |
| `--ant` | `claude-sonnet-4-6` | `ANTHROPIC_API_KEY` |
| `--ms` | `moonshot-v1-auto` | `MOONSHOT_API_KEY` |
| `--zai` | `glm-4-flash` | `ZAI_API_KEY`, then `ZHIPU_API_KEY` |
| `--qw` | `qwen-plus` | `DASHSCOPE_API_KEY`, then `QWEN_API_KEY` |
| `--oa` | `gpt-4o` | `OPENAI_API_KEY` |
| `--groq` | `llama-3.3-70b-versatile` | `GROQ_API_KEY` |
| `--ollama` | `deepseek-r1` | `OLLAMA_API_KEY` (optional) |

URL precedence is `--api`/`--base-url`, `CALLM_BASE_URL`, then `STRAITLY_BASE_URL`
for `--st` or `OPENAI_BASE_URL` for `--oa`, then the preset URL. Model precedence is
`--model`/`-m`, `CALLM_MODEL`, then `STRAITLY_MODEL` for `--st` or `OPENAI_MODEL` for
`--oa`, then the preset model (or the `--claude` shortcut). The models above are
CLI defaults; availability is controlled by each provider.

Nonempty environment variables take precedence over files. Files fill missing or
empty variables in order: current-directory `.env`, `.env` one directory above the
executable's directory, `~/.config/callm/config`, then legacy
`~/.config/straitly/config`. Files accept `KEY=value` or `export KEY=value` and
optional surrounding quotes; shell expansion is not performed.

---

## Usage Examples

### Quick Query (Default Model)

```bash
callm "Explain quantum entanglement in 2 sentences"
```

### Switch Provider Presets

```bash
# OrcaRouter automatic routing (ORCA_API_KEY); --or still selects OpenRouter
callm --orca --stats "Explain this error"
callm --orca models
callm --orca --claude --effort high "Review this function"

# Direct DeepSeek API
callm --ds "Write an LRU cache in Go"

# Claude Sonnet 4.6 (via Straitly/OpenRouter gateway)
callm --claude "Explain OKLCH color space"

# Direct Anthropic API with extended thinking
callm --ant --effort=high "Prove the Riemann hypothesis"

# Moonshot AI (Kimi) & Alibaba Cloud (Qwen)
callm --ms "Summarize recent breakthroughs in quantum computing"
callm --qw -m qwq-32b "Solve this math problem"

# OpenAI o3-mini reasoning model
callm --oa -m o3-mini --effort=medium "Implement an A* pathfinding algorithm"

# Fast open-source models via Groq
callm --groq -m llama-3.3-70b-versatile "Explain Rust lifetimes"

# Local Ollama (auto-detects inline <think> tags)
callm --ollama "Solve 17 * 23 step by step"
```

### HTTP User-Agent

Every API request sends `User-Agent: CallM (Call-LLM; +https://github.com/steamvogue/callm)`
by default, including streaming, Anthropic, catalog pagination, and raw requests.
Set `CALLM_USER_AGENT` for a persistent override (also supported in configuration
files), or use `--user-agent` for one command. Precedence is explicit flag,
nonempty environment value, then the project default. An explicit empty flag
omits the header; control characters are rejected before sending a request.

```bash
callm --user-agent "MyApp/1.0" "Hello"
CALLM_USER_AGENT="MyApp/1.0" callm --orca models
callm --user-agent "" --orca "Hello"
```

### OrcaRouter

`--orca` uses `ORCA_API_KEY`, base URL `https://api.orcarouter.ai/v1`, and
`orcarouter/auto` by default. Override the model with `-m`; `--orca --claude`
selects `anthropic/claude-sonnet-4.6`. `--or` continues to select OpenRouter.
The same key/URL/model precedence and 300-second timeout apply to chat, streaming,
model discovery, and raw requests.

`--effort` passes the documented `reasoning_effort` field for OrcaRouter to
translate; support depends on the selected model. Use it instead of
`--thinking-budget`, which this preset rejects. `--stats` opts into billed cost
with `X-OrcaRouter-Include-Cost: true`, reads `usage.cost_usd`, and requests final
streaming usage. Cost remains unavailable when omitted by the server.

See OrcaRouter’s [API reference](https://docs.orcarouter.ai/api-reference/chat/create-a-chat-completion),
[reasoning guide](https://docs.orcarouter.ai/advanced/reasoning), and
[cost reporting](https://docs.orcarouter.ai/operations/per-request-cost).

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
# Set reasoning effort:
callm --effort=high "Which is larger: 9.11 or 9.9?"

# Only show provider-returned reasoning on stderr (when available):
callm --only-reasoning "Which is larger: 9.11 or 9.9?"

# Suppress reasoning (final answer only):
callm --no-reasoning "What is the capital of France?"
```

### Multimodal Vision Models

```bash
callm --ant --image ./chart.png "Explain the trend shown in this chart"
```

### Model Discovery & Inspection

```bash
# Filter models by name/regex
callm models deepseek
callm models claude
callm --oa models gpt

# Inspect technical specs, context length, and pricing
callm info deepseek/deepseek-v4-flash-0731
```

---

## CLI Reference

<!-- CLI-HELP:START -->
```text
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

```
<!-- CLI-HELP:END -->

All API commands use a **300-second request timeout** by default, including
streaming, Anthropic, model discovery, and raw requests. The timeout covers
connection setup through the end of the response body; it is an overall limit,
not an idle timer. It starts when the HTTP request begins, after local input is read.
Set `--timeout 600` or `--timeout 10m` for a longer call, or `--timeout 0` to disable
the request limit. Cancellation signals interrupt HTTP requests and stdin waits. Header and stream-idle limits inherit `--timeout` unless explicitly overridden.
The separate stdin limit defaults to 300 seconds. Zero disables the selected limit;
`--timeout 0` also disables inherited header/idle limits, while explicit overrides remain active.

```bash
callm --timeout 10m "Solve this problem"
callm models --timeout 60s deepseek
callm info --timeout 60s deepseek/deepseek-v4-flash-0731
callm raw --timeout 300 /chat/completions '{"model":"example","messages":[]}'
```

---

## Installation

### Option 1: Download Precompiled Release Binaries

Download the latest standalone binary for your architecture from the [GitHub Releases](https://github.com/steamvogue/callm/releases) page:

- **Linux**: `callm-<version>-linux-amd64.tar.gz` or `callm-<version>-linux-arm64.tar.gz`
- **macOS**: `callm-<version>-darwin-arm64.tar.gz` (Apple Silicon) or `callm-<version>-darwin-amd64.tar.gz` (Intel)
- **Windows**: `callm-<version>-windows-amd64.zip`

Archives include the README, changelog, license, and `skills/callm/SKILL.md`.

Extract and move `callm` to your `PATH` (e.g. `/usr/local/bin`):

```bash
tar -xzf callm-*-linux-amd64.tar.gz
sudo mv callm /usr/local/bin/
```

### Option 2: Build From Source

Requirements: Go 1.22+

```bash
git clone https://github.com/steamvogue/callm.git
cd callm

# Build and install system-wide
sudo make install

# Or install for current user only ($HOME/.local/bin)
make install PREFIX=$HOME/.local
```

Verify the installation and version:

```bash
callm --version
```

## Option and failure behavior

- Provider keys are isolated. Resolution is explicit key, explicit key-env, global
  `CALLM_API_KEY`, then the selected provider's key and documented aliases.
  Missing keys do not fall back to other providers. Requests reject redirects to
  a different origin, including HTTPS-to-HTTP redirects.
- Provider flags work before or after a subcommand. For example,
  `callm --oa models gpt` and `callm models --oa gpt` are equivalent. Flags precede
  positional prompts/filters. For a provider proxy, keep its preset explicit, such as
  `--ant --api https://proxy.example/v1` or `--or --api https://proxy.example/v1`. Use `callm chat "models"` for a literal command-name prompt.
- Choose one provider, one token-limit flag, and either `--effort` or
  `--thinking-budget`. Conflicting output flags and invalid numbers are rejected.
  Unsupported provider combinations fail explicitly rather than silently dropping settings.
- Anthropic defaults to `claude-sonnet-4-6`; the gateway Claude shortcut uses
  `anthropic/claude-sonnet-4.6`. Anthropic supports image conversion and top-p.
  `--json-object` is an OpenAI-compatible option and is rejected for Anthropic;
  use `raw` with that provider's structured-output schema. Anthropic thinking budgets must
  be at least 1024 and below an explicit total token cap. An explicitly supplied cap is never raised automatically. On o-series models, use a completion limit and omit temperature.
- Streaming defaults to terminal stdout. `--stream=false` disables it explicitly.
  Reasoning is written to stderr and answers to stdout in both modes.
  `--json` preserves the original provider JSON; `--json --stats` retains stderr stats.
- Nonempty delayed pipes are read through EOF on every platform. Use `--no-stdin`
  when an inherited pipe is intentionally left open. Stdin, local attachments, and buffered HTTP
  responses are limited to 64 MiB; SSE events are limited to 8 MiB.
- HTTP errors, API error envelopes, malformed/truncated SSE, and cancellation
  return failure exit codes. Missing usage and catalog prices display as unavailable
  or unknown, rather than zero. No automatic generation retries are performed.
- `make test-live` requires exported credentials and performs actual billed calls.
  It reports passed assertions and skipped providers, includes Anthropic, and exits
  2 if no tests ran. `make test-race` requires a host supported by ThreadSanitizer;
  CI runs race detection on Linux amd64.
