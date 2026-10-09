---
name: callm
description: Use the fast, standalone `callm` CLI tool to query external LLMs (DeepSeek V4, Claude, OpenAI, OpenRouter, OrcaRouter, Kimi Code, etc.) for second opinions, complex logic, code review, or specialized model capabilities.
---

# callm (call - llm) Agent Skill

`callm` is a fast standalone CLI utility installed at `/usr/local/bin/callm` (and in `$PATH`).
It connects to OpenAI-compatible gateways and the native Anthropic API, with millisecond startup and real-time streaming. These instructions describe v0.10.0; check `callm --version` and `callm --help` when using an older installation.

Default provider: **Poolside** (`poolside/laguna-s-2.1`). Without a preset flag,
callm uses the first provider whose key is set, checked in order Poolside,
OrcaRouter, Straitly, DeepSeek, OpenRouter, Kimi Code; if none is set, Poolside.

## When to Use This Skill

- When you need a second opinion on a complex architectural decision, bug, or algorithm.
- When you want to delegate a reasoning task to DeepSeek (`--ds`; `deepseek-flash` thinks by default, displayed with `--reasoning`).
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
# Straitly gateway:
callm --st "Your prompt here"

# Claude Sonnet 4.6 shortcut (via Straitly/OpenRouter gateway):
callm --claude "Refactor this SQL query"

# Direct Anthropic API with extended thinking:
callm --ant --effort=high "Prove the Riemann hypothesis"

# Direct DeepSeek API:
callm --ds "Write an optimized LRU cache in Go"

# Kimi Code subscription (KIMI_API_KEY), default model kimi-for-coding:
callm --kimi -f main.go "Review this code for bugs"

# Moonshot AI (pay-as-you-go) & Alibaba Cloud (Qwen):
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

# Poolside (POOLSIDE_API_KEY), default model poolside/laguna-s-2.1:
callm --pool "What are channels in Go?"

# Local Ollama or vLLM (use explicit inline <think> parsing):
callm --ollama --parse-think "Solve 17 * 23 step by step"
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

### 6. Model Discovery, Specs & Exports

```bash
callm models deepseek
callm --or info deepseek/deepseek-v4-flash-0731

# Substring filter terms (comma-separated, case-insensitive; z.ai matches z-ai):
callm models --filter="deepseek,z.ai,qwen"

# Lossless JSON catalog (unknown provider fields preserved):
callm models --format=json --filter="deepseek" > models.json

# Paste-ready provider configs: zed (default target), kilo, continue
callm models --format=zed
callm models --format=kilo --provider-name "My Gateway"
callm models --format=continue
```

`--json` aliases `--format=json`; `vscode` aliases `--format=continue`. Exports
never include API keys: Zed/Kilo read `<PROVIDER_ID>_API_KEY`, Continue uses its
own `apiKey` setting. Zed output omits `max_tokens` (with a stderr warning) for
models that do not report a context length.

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

callm ignores a `.env` in the current directory unless `CALLM_LOAD_DOTENV=1` is set
in the environment or `~/.config/callm/config`, because a checkout's `.env` could
redirect API keys (for example with `CALLM_BASE_URL`). Do not enable it inside
untrusted repositories; export keys or use `~/.config/callm/config` instead.
`CALLM_LOAD_DOTENV=0` hides the stderr notice printed when such a file is skipped.
Releases before v0.9.0 load the current-directory `.env` automatically.

OrcaRouter uses `--orca` and `ORCA_API_KEY`; `--or` remains OpenRouter. Its base
URL is `https://api.orcarouter.ai/v1`. `--effort` sends `reasoning_effort` for the
gateway to translate; support depends on the model. `--thinking-budget` is
rejected on this preset. `--stats` opts into `usage.cost_usd` using the
`X-OrcaRouter-Include-Cost` header, including final usage on streams.

All API requests default to `User-Agent: CallM (Call-LLM; +https://github.com/steamvogue/callm)`.
Override with `--user-agent "MyApp/1.0"` or `CALLM_USER_AGENT`; explicit flags win
and an empty environment value keeps the default. `--user-agent ""` omits the
header. This applies to every provider and API command; control characters fail.

Kimi Code subscription support: `--kimi` uses `KIMI_API_KEY`, base URL
`https://api.kimi.com/coding/v1`, and model `kimi-for-coding` through the
OpenAI-compatible protocol. Use `-m` for another model available to the
subscription. This flag previously aliased Moonshot; `--ms`/`--moonshot` still
use `MOONSHOT_API_KEY` and the separate pay-as-you-go Moonshot endpoint.
The existing key/URL/model precedence applies. `--stats` reports returned token
usage and only server-supplied cost, without estimating subscription charges.
Requests retain callm's default User-Agent identity. See the
[Kimi Code docs](https://www.kimi.com/code/docs/en/) for membership/model access.
Check `callm --help` for `--kimi` subscription support before using older binaries.

Poolside uses `--pool` and `POOLSIDE_API_KEY`, base URL
`https://inference.poolside.ai/v1`, and default model `poolside/laguna-s-2.1`
through the OpenAI-compatible protocol. Use `-m` for another Poolside model.
The usual key/URL/model precedence and User-Agent identity apply.

DeepSeek Direct uses `--ds` and `DEEPSEEK_API_KEY`, base URL
`https://api.deepseek.com`, and default model `deepseek-flash` (DeepSeek V4.1
Flash); use `-m deepseek-v4-pro` for V4 Pro. DeepSeek retired `deepseek-chat` and
`deepseek-reasoner` after 2026-07-24, and callm v0.8.0 and earlier still default
`--ds` to `deepseek-chat`, so pass `-m deepseek-flash` with older binaries.
`deepseek-flash` thinks by default: reasoning returns as `reasoning_content`,
`--effort` sends `reasoning_effort` (DeepSeek maps `medium` to `high`), and
`--thinking-budget` is rejected. For a non-thinking request, use
`callm --ds raw /chat/completions` with `"thinking": {"type": "disabled"}` in the body.

## Result and model behavior in v0.10.0

Check `callm --help` before using these options with an older binary.

- Text output rejects truncation/refusal/tool-dependent/empty results. `--allow-empty`
  permits an expected empty result; `--strict` also requires a terminal reason.
  Use `--json --strict` for pipelines; plain `--json` preserves diagnostic envelopes
  with transport success. Stage streamed output because partial bytes can precede failure.
- Literal `<think>` text is preserved. Use `--parse-think` only for a model that
  embeds reasoning in those tags. It conflicts with full `--json`. Reasoning fields
  keep their display controls. Write/flush/statistics errors return failure.
- Anthropic input totals include uncached, cache-read and cache-creation tokens once;
  cache categories are displayed as included in input. Compatible-provider totals
  keep provider accounting; missing cost stays unknown.
- Native Claude 4.6 and recognized newer IDs use adaptive thinking with `--effort`
  and `output_config.effort`. 4.6 supports low/medium/high/max; recognized 5.x and
  Opus 4.7–4.8 add xhigh and reject manual budgets/sampling. Older/custom models
  keep legacy low/medium/high budgets. Explicit caps remain intact. Returned thinking
  may be omitted; local display flags do not alter inference or cost.
- Known GPT-6 Astra/6.1 Sol/Sol/Luna IDs normalize token caps and accept model-specific
  efforts. Sol/Luna accept none and allow sampling only with explicit none;
  Astra/6.1 Sol reject none/minimal and sampling. Unknown/custom models retain
  low/medium/high and explicit caps; use raw for other model options.
- Native Anthropic editor exports fail with native-adapter guidance; table/JSON
  catalogs remain available. Compatible gateway exports remain supported.
- Missing-key errors show preset flags/variable names without credentials. Select
  providers explicitly in pipelines; automatic priority remains as documented.
- The repository tests supported Go 1.26/1.27 lines; releases compile with 1.27.2.
  The guarded example and free live-test guards are fixture-tested. `make test-live`
  requires a current zero-price OpenRouter `:free` catalog entry and makes at most
  two calls, without retries or paid fallback. Never use paid models in tests.
  The free smoke default is `google/gemma-4-26b-a4b-it:free`; production defaults
  remain unchanged. Six synthetic tasks are insufficient for default promotion.

### Pipeline inputs, schemas and results (v0.10.0)

Check binary help for these options before using an older installation:

```bash
callm --or -m google/gemma-4-26b-a4b-it:free --no-stdin \
  --prompt-file prompt.txt --system-file system.txt \
  --validate-schema output.schema.json --result-json --max-tokens 2048
callm raw --or --body-file request.json /chat/completions
```

`--prompt-file` reads plain text retaining whitespace, after files/stdin and before
positional text. `--system-file` reads verbatim instructions and conflicts with
`--system`. Regular files only; raw `--body-file -` reads stdin with its timeout.
Raw requires exactly one valid JSON body. Chat/raw `--max-input-bytes` caps the
combined serialized request (default/maximum 67108864), including escaped text,
schema and encoded images; it is a byte limit, not a token/context estimate.

`--schema FILE` requests provider constraints and validates locally; native
Anthropic uses `output_config.format`, compatible providers use strict
`response_format.json_schema`. Provider/model subsets apply without silent
rewriting. `--validate-schema FILE` validates only locally. Schemas are at most
1 MiB; OpenRouter --schema also requires provider parameter support for routing.
Default Draft 2020-12, declared older supported drafts, built-in format
assertions. Use in-document `$defs`; external schema resources are blocked.
Both options buffer output, imply strict completion checks and conflict with
streaming/json-object/parse-think/only-reasoning. Checks also precede raw `--json`
publication. Schema validity does not establish task correctness.

`--result-json` emits a version 1 status/answer/requested/returned-model,
finish/refusal/usage/duration/schema/error envelope; it buffers and implies strict
completion checks. It conflicts with --json, streaming, reasoning display controls
and --allow-empty. Require exit 0 **and** status ok. Request/completion/schema
errors produce status error; preflight/input/credential failures may have empty
stdout. Error answers can be partial, absent usage/schema checks are null, and
reported cache/reasoning counts are included categories, never added twice.

The repository's `examples/mini-harness.py` (optional Python 3, Linux/macOS) runs
a version 1 manifest with 1–16 sequential calls, local schemas, atomic artifacts,
journal/hash resume and no automatic retries or model-controlled tools. Pin the
API/model and keep credentials outside the manifest. Its example selects a free
model; the general runner does not enforce prices. Use localhost mocks in tests.
See the [manifest contract](https://github.com/steamvogue/callm/blob/main/examples/README.md).
The installed CLI itself needs no external runtime; builds use pinned Go modules.
