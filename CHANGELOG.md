# Changelog

## v0.10.0 — 2026-10-09

- Add regular-file prompt/system inputs, raw `--body-file FILE|-`, and a combined
  64 MiB request limit with a lower `--max-input-bytes` override. Raw bodies now
  require one valid JSON value and reject extra/conflicting body arguments.
- Add `--schema` provider constraints plus offline JSON Schema validation, and
  `--validate-schema` for local-only checks. Buffer and validate before publishing;
  block external schema references. Builds now use pinned Go modules for validation.
  OpenRouter schema requests require provider parameter support; local validation
  remains mandatory even after a successful API response.
- Add strict, versioned `--result-json` status/answer/model/finish/refusal/usage/timing
  envelopes, including request/completion/schema errors. Keep original `--json`.
- Add an optional Python mini harness with sequential manifests, atomic artifacts,
  hashes/journal/resume and one call per step. Test the harness against localhost
  mocks in CI; no automatic retries, repairs or model-controlled tools.
- Make live tests free-only: verify all catalog prices are zero before at most two
  explicit OpenRouter `:free` calls, with no paid fallback. Record a separate
  24-call free-model evaluation; leave production provider/model defaults unchanged.
- Remove the Go race runtime's repeated one-second exit delay from CLI fixture
  subprocesses while retaining detector settings, keeping the expanded CI suite
  within its existing 60-second timeout.

- Reject truncated, refused, filtered, tool-dependent and empty text completions.
  Add `--strict` for terminal-reason enforcement (including JSON), and
  `--allow-empty` for intentionally empty answers. Plain `--json` retains diagnostic
  envelope semantics. Partial streamed output can precede a failure.
- Preserve literal `<think>` content by default; add explicit `--parse-think`.
  Propagate answer/reasoning/statistics/raw/info/table/help/version write errors,
  including renderer short writes and flush failures.
- Include Anthropic cache reads and cache creation in processed-input statistics
  without adding cache categories again to compatible-provider totals.
- Add bounded model profiles for native Claude adaptive thinking/effort and known
  GPT-6 reasoning, sampling and token-limit parameters. Keep provider/model defaults;
  explicit caps remain unchanged. Legacy/manual and adaptive effort can consume
  different amounts of reasoning; evaluate the new behavior on your workload.
- Reject native Anthropic editor exports with native-adapter guidance; retain
  table and lossless JSON catalogs. Improve redacted missing-key diagnostics.
- Test supported Go 1.26/1.27 patches in CI; build releases with Go 1.27.2.
  Keep the Go 1.22 module language minimum.
- Correct provider-specific examples and add guarded JSON publication. Free-only
  live tests require valid answer/usage/finish metadata and zero reported cost.
- Record individual repairs, observations and validation in
  `audit/2026-10-09/PROGRESS.md`; use unit/local mock tests and synthetic fixtures.
  Paid provider generation and editor imports were not executed. Separate
  free evaluation/pipeline progress follows in `audit/2026-10-09/PIPELINE_PROGRESS.md`.

## v0.9.0 — 2026-09-16

### Security

- A `.env` in the current directory is no longer loaded by default. Previously a
  checked-out repository could set `CALLM_BASE_URL` (or any other variable) in its
  `.env` and receive the API key callm resolved from your environment or
  configuration files. Set `CALLM_LOAD_DOTENV=1` in the environment,
  `~/.config/callm/config`, or the executable-relative `.env` to load it again; it
  keeps its previous precedence and cannot enable itself.
- callm prints a stderr notice when it skips a current-directory `.env` that
  defines callm variables; `CALLM_LOAD_DOTENV=0` hides it. Invalid values fail
  before any request. Special files such as named pipes are skipped instead of
  blocking startup.

### DeepSeek

- Changed the DeepSeek Direct (`--ds`) default model from `deepseek-chat` to
  `deepseek-flash` (DeepSeek V4.1 Flash). DeepSeek retired the legacy
  `deepseek-chat` and `deepseek-reasoner` model names after 2026-07-24; earlier
  callm releases need `-m deepseek-flash`.
- `deepseek-flash` thinks by default, unlike the former non-thinking
  `deepseek-chat`, so `--ds` responses now include `reasoning_content` under the
  usual display rules. `--effort` still sends `reasoning_effort`; send
  `"thinking": {"type": "disabled"}` through `raw` for a non-thinking request.
- The optional live DeepSeek checks use `deepseek-flash`, and the minimal check
  allows 1024 tokens instead of 64 so default thinking has room to finish.

### Compatibility and validation

- Projects that kept keys or endpoints in a current-directory `.env` must export
  them, move them to `~/.config/callm/config`, or opt in with
  `CALLM_LOAD_DOTENV=1`. The `.env` one directory above the executable (such as a
  repository checkout's `bin/callm`) still loads.
- Added local mock coverage for current-directory `.env` isolation, opt-in and
  opt-out, invalid values, and named pipes, plus the DeepSeek default model, model
  override, effort, and raw thinking switch. Updated CLI help, README, agent
  skill, and installed skill copies.

## v0.8.0 — 2026-09-11

- Added `callm models` catalog export. `--format=json` (alias `--json`) saves the
  filtered raw provider catalog losslessly, preserving provider-specific fields;
  `--format=zed`, `--format=kilo`, and `--format=continue` (`vscode` alias) print
  paste-ready provider configuration fragments for those tools.
- Added `--filter` to `callm models`: comma-separated, case-insensitive substring
  terms with separator folding, so `z.ai` matches `z-ai`. Terms are OR-combined and
  AND-combined with the existing positional regex filter.
- Added `--provider-name` to override the provider id used by `zed`/`kilo` output.
  Exports never include API keys; models missing a context length are exported
  without Zed's required `max_tokens` and warn on stderr.
- Updated CLI help, README, agent skill, and installed skill copies.

## v0.7.1 — 2026-09-09

- Changed the default provider from Straitly to Poolside. Without an explicit
  preset flag, callm selects the first provider whose key is set, checked in
  order Poolside, OrcaRouter, Straitly, DeepSeek, OpenRouter, Kimi Code; if none
  is set, Poolside is used. `--st` still selects Straitly explicitly.
- Updated CLI help, README, agent skill, and Makefile key checks.

## v0.7.0 — 2026-09-09

- Added Poolside support with `--pool`, `POOLSIDE_API_KEY`, base URL
  `https://inference.poolside.ai/v1`, and default model `poolside/laguna-s-2.1`.
  Uses the OpenAI-compatible client for chat, streaming, models, info, and raw
  requests, preserving the usual configuration overrides and callm User-Agent.
- Updated CLI help, README, agent skill, and optional live-test provider list.

## v0.6.0 — 2026-09-06

- Added Kimi Code subscription support with `--kimi`, `KIMI_API_KEY`, base URL
  `https://api.kimi.com/coding/v1`, and default model `kimi-for-coding`.
  Uses the OpenAI-compatible client for chat, streaming, models, info, and raw
  requests, preserving the usual configuration overrides and callm User-Agent.
- Changed `--kimi` from a Moonshot alias to the subscription preset. Existing
  Moonshot users should use `--ms` or `--moonshot` with `MOONSHOT_API_KEY`.
- Subscription stats use server-returned usage/cost only; no per-call price is
  inferred. Added local mock coverage, CLI help, examples, agent instructions,
  and an optional live-test entry.

## v0.5.0 — 2026-09-06

- All API requests now identify the project with the default User-Agent
  `CallM (Call-LLM; +https://github.com/steamvogue/callm)`.
- Added `--user-agent` to chat, models, info, and raw commands, with
  `CALLM_USER_AGENT` as the environment/configuration-file default override.
  Explicit flags win; an empty environment value keeps the project default.
  `--user-agent ""` omits the header; control characters are rejected.
- Updated CLI help, README examples, and agent skills.

## v0.4.0 — 2026-09-06

- Added OrcaRouter with `--orca`, `ORCA_API_KEY`, base URL
  `https://api.orcarouter.ai/v1`, and default model `orcarouter/auto`.
  `--or` remains OpenRouter. All existing timeout defaults and overrides apply.
- Supports chat, SSE streaming, models, info, raw requests, and `--orca --claude`.
  `--effort` passes OrcaRouter’s `reasoning_effort`; `--thinking-budget` rejects
  with guidance to use effort instead. Model capabilities remain provider-dependent.
- OrcaRouter `--stats` requests billed cost through `X-OrcaRouter-Include-Cost`
  and reads `usage.cost_usd`, including streaming usage.
- Updated CLI help, README, agent skill, and optional live-test provider list.

## v0.3.0 — 2026-09-05

This release fixes stalled calls, dropped input, silently ignored options, and
misleading success results across chat, streaming, model discovery, and raw requests.

### Timeouts and input

- All API calls default to a **300-second total timeout**. `--timeout` accepts
  seconds or durations (`600`, `10m`); `0` disables it.
- Added `--header-timeout` and streaming chat `--idle-timeout`, both inheriting
  `--timeout` unless explicitly overridden. Zero disables each selected limit.
- Added independent `--stdin-timeout` (default **300 seconds**) and `--no-stdin`.
  Pipes are read through EOF on all platforms, including delayed producers;
  cancellation interrupts both input and HTTP waits.
- Bound input files/stdin/buffered responses to 64 MiB and SSE events to 8 MiB.

### Correctness and provider support

- Parse full SSE events, reject malformed or truncated streams, and stop Anthropic
  streams immediately at `message_stop`. HTTP and API errors return failure.
- Honor configured clients and endpoint overrides consistently. Follow Anthropic
  model catalog pagination under one deadline and detect repeated cursors.
- Update Claude defaults to `claude-sonnet-4-6` directly and
  `anthropic/claude-sonnet-4.6` through Straitly/OpenRouter.
- Convert Anthropic images correctly, preserve top-p, validate thinking budgets,
  and preserve explicit token caps. Unsupported JSON-object requests fail explicitly.
- Apply reasoning display controls to both output modes. Preserve original provider
  JSON and large numbers; `--json --stats` retains separate stderr statistics.
- Request streaming usage for `--stats`; handle string costs and unknown usage,
  prices, and context sizes without reporting missing data as zero.
- Accept provider flags before subcommands and honor `--stream=false`. Reject
  malformed numbers, conflicting flags, and unsupported provider combinations.
  Normalize token limits for OpenAI o-series models.

### Compatibility changes

- Missing provider keys no longer fall back to another provider's credentials.
  Use the selected provider's key, `CALLM_API_KEY`, or an explicit key override.
  Cross-origin redirects are rejected before credentials reach the destination.
- Streaming and reasoning display default on only when stdout is a terminal.
  Reasoning goes to stderr; final answers go to stdout.
- Automation relying on incomplete streams, silently dropped settings, or API
  errors returning success must now handle nonzero exit codes.

### Documentation and validation

- Synchronize CLI/subcommand help, README, examples, and the callm agent skill with
  current options, aliases, defaults, and configuration precedence. Add a README/help
  consistency test and documented maintenance rules.
- Add mock regression coverage and 91 local CLI scenarios; improve live-test
  assertions and expose `make test-race`, enabled in Linux amd64 CI.
- Release archives cover Linux amd64/arm64, macOS amd64/arm64, and Windows amd64,
  with SHA256 checksums. Mock validation does not establish live provider generation
  quality, billing, or account-specific model availability.
