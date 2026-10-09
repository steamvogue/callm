# Project maintenance

- Never fabricate validation results. Check behavior using local mock tests before making claims; distinguish live provider validation from mocks and compilation from native execution.
- Whenever product behavior, options, aliases, defaults, model IDs, or configuration precedence change, update all related documentation in the same change: CLI help, every applicable README, examples, release notes, and `skills/callm/SKILL.md`. Synchronize installed callm agent skill copies when working on a host where they are present. Inspect related metadata and configuration examples for stale values.
- Keep README's `CLI-HELP` block identical to the reference section of `callm --help`; `TestREADMEHelpReference` enforces this. Check each subcommand's help too.
- Run `go test ./...`, `go vet ./...`, and relevant behavioral probes after changes. Use `make test-race` on supported hosts. `make test-live` verifies current zero-price OpenRouter `:free` models before making at most two calls; no paid models, retries or fallback are allowed in tests. Ordinary tests use mocks.
- This is a public repository. Keep local audit reports, host inventories, developer identities, filesystem locations and infrastructure details in the ignored `audit/` folder. Do not commit them or link public documentation/release notes to them. Ignoring a path does not remove previously published Git history.
- Store verified useful local tooling in the ignored audit folder for reuse. Prefer `rg` for search, Go for builds/tests, `gh` for GitHub releases, `actionlint` for workflows, ShellCheck for shell scripts, and `hyperfine` for startup benchmarks. Run race checks on supported hosts; Linux amd64 CI provides race coverage.

- A verified DuckDuckGo research CLI supports `search --format=json` and `fetch URL --format=text`; record machine-specific availability privately.

- Useful pipeline tooling: `htmlmd --profile plain-text --extract-selector main` extracts local HTML; `jq` validates response envelopes and model catalogs; `qsv` handles tabular data. Prefer `ast-grep` when the `sg` alias is deprecated. Record verified versions and host locations privately.

- Official Go toolchains can be selected with `GOTOOLCHAIN`; use writable, separate caches when needed and record host-specific validation privately.
- Python 3 standard-library harness checks are `scripts/test_mini_harness.py` (localhost mocks) and `scripts/test_free_live.py` (fake CLI, no network). `sha256sum` verifies installed binary/skill copies. If `gh run list` lacks `--commit`, use `gh api repos/steamvogue/callm/actions/runs` and filter by full head SHA instead.
- CLI fixture subprocesses must retain `cliRaceEnvironment()` when replacing cmd.Env. It preserves caller GORACE settings and appends `atexit_sleep_ms=0`; otherwise every successful instrumented child incurs a one-second exit delay and can exhaust the suite's 60-second CI timeout. Race instrumentation remains enabled in amd64 CI.
