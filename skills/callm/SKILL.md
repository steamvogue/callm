---
name: callm
description: Use the fast, zero-dependency `callm` CLI tool to query external LLMs (DeepSeek V4, Claude, OpenAI, OpenRouter, OrcaRouter, etc.) for second opinions, complex logic, code review, or specialized model capabilities.
---

# callm (call - llm) Agent Skill

`callm` is a fast standalone CLI utility installed at `/usr/local/bin/callm` (and in `$PATH`).
It connects to OpenAI-compatible gateways and the native Anthropic API, with millisecond startup and real-time streaming. These instructions describe v0.4.0+; check `callm --version` and `callm --help` when using an older installation.

Default model: **`deepseek/deepseek-v4-flash-0731`**.

## When to Use This Skill

- When you need a second opinion on a complex architectural decision, bug, or algorithm.
- When you want to delegate a reasoning task to DeepSeek V4 / R1 (`--reasoning` or `--ds`).
- When you need access to models from Anthropic, Meta, or OpenAI via OpenRouter (`--or`).
- When analyzing large files or diffs with piped input.

## Command Syntax & Patterns

### 1. Direct Queries

```bash
# Simple fast query (clean stdout, no thinking block):
callm --no-reasoning "What are the edge cases of binary search?"

# Display provider-returned reasoning on stderr when available:
callm --reasoning "Explain why 9.11 is smaller than 9.9"
```

### 2. Provider Presets & Reasoning

```bash
# Straitly gateway (default):
callm "Your prompt here"

# Claude Sonnet 4.6 shortcut (via Straitly/OpenRouter gateway):
callm --claude "Refactor this SQL query"

# Direct Anthropic API with extended thinking:
callm --ant --effort=high "Prove the Riemann hypothesis"

# Direct DeepSeek API:
callm --ds "Write an optimized LRU cache in Go"

# Moonshot AI (Kimi) & Alibaba Cloud (Qwen):
callm --ms "Summarize recent developments in AI"
callm --qw -m qwq-32b "Solve this math problem"

# OpenAI o3-mini reasoning model:
callm --oa -m o3-mini --effort=medium "Solve this competitive programming problem"

# OpenRouter with specific model:
callm --or -m anthropic/claude-sonnet-4.6 "Refactor this query"

# OrcaRouter (ORCA_API_KEY), default model orcarouter/auto:
callm --orca --stats "Explain this error"
callm --orca models
callm --orca --claude --effort high "Review this function"

# Local Ollama or vLLM (auto-detects inline <think> tags):
callm --ollama "Solve 17 * 23 step by step"
```

### 3. Piping Code or Stdin

```bash
# Pipe file context:
cat main.go | callm "Audit this code for race conditions"

# Pipe git diff for commit message:
git diff | callm --no-reasoning "Write a concise conventional commit message"
```

### 4. Context Files & Modalities

```bash
# Attach files into prompt context:
callm -f schema.sql "Generate 5 sample INSERT statements"

# Attach image for vision models:
callm --ant --image ./chart.png "Summarize this chart"
```

### 5. API Key Overrides

```bash
# Point to a custom environment variable name:
callm --api-key-env=CUSTOM_TOKEN "Prompt"

# Explicit bearer key:
callm --api-key="sk-..." "Prompt"
```

### 6. Model Discovery & Specs

```bash
callm models deepseek
callm info deepseek/deepseek-v4-flash-0731
```

### 7. Timeouts and output defaults

All API commands (`chat`, `models`, `info`, `raw`) default to a **300-second total
request timeout**. `--timeout` accepts seconds or a Go duration (`600`, `10m`);
`0` disables it. `--header-timeout` inherits `--timeout` unless explicitly set.
For streaming chat, `--idle-timeout` also inherits `--timeout` and measures time
between received bytes. Zero disables the selected limit, including inherited
header/idle limits when `--timeout 0` is used.

Piped stdin is read through EOF before the API call. Its independent
`--stdin-timeout` defaults to **300 seconds**; `0` disables it. Use `--no-stdin`
when an inherited pipe intentionally stays open. Signals cancel input and HTTP.

```bash
callm --timeout 10m --idle-timeout 60s --stream "Analyze this problem"
callm --stdin-timeout 30 --timeout 300 "Summarize piped input"
callm models --timeout 60s deepseek
```

Streaming and reasoning display default on when stdout is a terminal and off
otherwise. `--stream=false`/`--no-stream` disable streaming. Answers go to stdout;
reasoning and `--stats` go to stderr. `--reasoning` controls display only;
`--effort` (alias `--reasoning-effort`) or `--thinking-budget` requests reasoning
where supported. `--json` emits the original non-streaming provider JSON.
Temperature (`-t`/`--temp`/`--temperature`), top-p and token caps are omitted unless
set, except Anthropic defaults to a 4096-token cap, adjusted for implicit thinking
budgets. Explicit caps are preserved; conflicting options fail.

Keys resolve as explicit key > named key-env > `CALLM_API_KEY` > selected provider
key/alias. Missing provider keys never fall back to another provider. URL and
model overrides resolve as explicit flag > `CALLM_*` > selected provider's
`STRAITLY_*`/`OPENAI_*` > preset. `--claude` uses `anthropic/claude-sonnet-4.6` on
Straitly/OpenRouter/OrcaRouter and `claude-sonnet-4-6` on Anthropic. Without an explicit
provider it selects Anthropic if `ANTHROPIC_API_KEY` is present and both
`STRAITLY_API_KEY` and `OPENROUTER_API_KEY` are absent. See `callm --help` and the
[root README](https://github.com/steamvogue/callm/blob/main/README.md) for all presets, aliases and configuration files.

OrcaRouter uses `--orca` and `ORCA_API_KEY`; `--or` remains OpenRouter. Its base
URL is `https://api.orcarouter.ai/v1`. `--effort` sends `reasoning_effort` for the
gateway to translate; support depends on the model. `--thinking-budget` is
rejected on this preset. `--stats` opts into `usage.cost_usd` using the
`X-OrcaRouter-Include-Cost` header, including final usage on streams.
