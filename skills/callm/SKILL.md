---
name: callm
description: Use the fast, zero-dependency `callm` CLI tool to query external LLMs (DeepSeek V4, Claude, OpenAI, OpenRouter, etc.) for second opinions, complex logic, code review, or specialized model capabilities.
---

# callm (call - llm) Agent Skill

`callm` is a fast standalone CLI utility installed at `/usr/local/bin/callm` (and in `$PATH`).
It connects to OpenAI-compatible gateways (Straitly, OpenRouter, DeepSeek Direct, or custom endpoints) with sub-3ms startup and real-time streaming.

Default model: **`deepseek/deepseek-v4-flash-0731`** (1M context, high speed, ultra-low cost).

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

# With DeepSeek chain-of-thought reasoning:
callm --reasoning "Explain why 9.11 is smaller than 9.9"
```

### 2. Provider Presets

```bash
# Straitly gateway (default):
callm "Your prompt here"

# Direct DeepSeek API:
callm --ds "Write an optimized LRU cache in Go"

# OpenRouter with specific model:
callm --or -m anthropic/claude-3.5-sonnet "Refactor this SQL query"

# Custom endpoint (e.g. local Ollama or vLLM):
callm --api=http://localhost:11434/v1 -m llama3 "Hello"
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
callm -m "deepseek/deepseek-v4-flash-vision-exp" --image ./chart.png "Summarize this chart"
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
