# callm (call - llm)

[![CI](https://github.com/steamvogue/callm/actions/workflows/ci.yml/badge.svg)](https://github.com/steamvogue/callm/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/steamvogue/callm)](https://github.com/steamvogue/callm/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

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
  - `--ant`, `--anthropic`: Anthropic Direct API (`https://api.anthropic.com/v1/messages`)
  - `--claude`: Claude 3.7 Sonnet shortcut across gateways
  - `--ms`, `--moonshot`, `--kimi`: Moonshot AI Kimi (`https://api.moonshot.cn/v1`)
  - `--zai`, `--glm`: Zhipu AI GLM / ZAI (`https://open.bigmodel.cn/api/paas/v4`)
  - `--qw`, `--qwen`: Alibaba Cloud DashScope Qwen (`https://dashscope.aliyuncs.com/compatible-mode/v1`)
  - `--oa`, `--openai`: OpenAI Direct API (`https://api.openai.com/v1`)
  - `--groq`: Groq Ultra-Fast OSS (`https://api.groq.com/openai/v1`)
  - `--ollama`: Ollama Local Gateway (`http://localhost:11434/v1`)
  - `--api=URL` / `--base-url=URL`: Custom OpenAI-compatible endpoint (vLLM, SGLang, etc.)
- **Real-Time SSE Streaming**: Native streaming with instant token delivery and graceful `Ctrl+C` handling.
- **Universal Chain-of-Thought (Reasoning)**:
  - Real-time rendering of thinking tokens before the main answer across **DeepSeek (R1/V3)**, **Claude 3.7 Sonnet**, **OpenAI o1/o3-mini**, **Qwen (QwQ)**, **Moonshot (Kimi)**, and **local OSS models**.
  - **Inline `<think>` Stream Parsing**: Automatically extracts and styles reasoning tokens from open-source models (Ollama, vLLM) that stream thinking tags inline.
  - Granular control via `--reasoning`, `--no-reasoning`, and `--only-reasoning`.
  - Configurable reasoning effort: `--effort=low|medium|high` and `--thinking-budget=N`.
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

# Claude 3.7 Sonnet (via Straitly/OpenRouter gateway)
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
callm --oa models gpt

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
  callm version | -v | --version
  callm -h | --help

Provider Presets:
  --st                             Straitly Gateway (default: https://api.straitly.ai/v1)
  --or                             OpenRouter Gateway (https://openrouter.ai/api/v1)
  --ds                             DeepSeek Direct API (https://api.deepseek.com)
  --ant, --anthropic               Anthropic Direct API (https://api.anthropic.com/v1)
  --claude                         Claude 3.7 Sonnet shortcut across gateways
  --ms, --moonshot, --kimi         Moonshot AI Kimi (https://api.moonshot.cn/v1)
  --zai, --glm                     Zhipu AI GLM / ZAI (https://open.bigmodel.cn/api/paas/v4)
  --qw, --qwen                     Alibaba DashScope Qwen (https://dashscope.aliyuncs.com/compatible-mode/v1)
  --oa, --openai                   OpenAI Direct API (https://api.openai.com/v1)
  --groq                           Groq Ultra-Fast OSS (https://api.groq.com/openai/v1)
  --ollama                         Ollama Local Gateway (http://localhost:11434/v1)
  --api, --base-url URL            Custom OpenAI-compatible endpoint URL

Chat Options:
  -m, --model MODEL                Model ID override (default: deepseek/deepseek-v4-flash-0731)
  -k, --key, --api-key KEY         API key value override
      --key-env, --api-key-env ENV Custom environment variable name containing API key
  -s, --system SYSTEM              System prompt instruction
  -t, --temp TEMPERATURE           Sampling temperature (e.g. 0.7, 0.0)
  -n, --max-tokens N               Maximum tokens to generate
      --max-completion-tokens N    Maximum completion tokens (for OpenAI o1/o3 reasoning models)
      --effort EFFORT              Reasoning effort: low, medium, high (OpenAI o1/o3, OpenRouter, Claude)
      --thinking-budget N          Extended thinking token budget (Claude 3.7 / OpenRouter)
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

### Option 1: Download Precompiled Release Binaries

Download the latest standalone binary for your architecture from the [GitHub Releases](https://github.com/steamvogue/callm/releases) page:

- **Linux**: `callm-<version>-linux-amd64.tar.gz` or `callm-<version>-linux-arm64.tar.gz`
- **macOS**: `callm-<version>-darwin-arm64.tar.gz` (Apple Silicon) or `callm-<version>-darwin-amd64.tar.gz` (Intel)
- **Windows**: `callm-<version>-windows-amd64.zip`

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
