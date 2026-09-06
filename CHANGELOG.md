# Changelog

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
