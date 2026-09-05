# Project maintenance

- Never fabricate validation results. Check behavior using local mock tests before making claims; distinguish live provider validation from mocks and compilation from native execution.
- Whenever product behavior, options, aliases, defaults, model IDs, or configuration precedence change, update all related documentation in the same change: CLI help, every applicable README, examples, release notes, and `skills/callm/SKILL.md`. Synchronize installed callm agent skill copies when working on a host where they are present. Inspect related metadata and configuration examples for stale values.
- Keep README's `CLI-HELP` block identical to the reference section of `callm --help`; `TestREADMEHelpReference` enforces this. Check each subcommand's help too.
- Run `go test ./...`, `go vet ./...`, and relevant behavioral probes after changes. Use `make test-race` on supported hosts. `make test-live` makes billed provider calls; ordinary tests use mocks.
- Store verified useful local tooling here for reuse: `rg` for search, Go for builds/tests, `gh` for GitHub releases, `actionlint` for workflows, ShellCheck for shell scripts, and `hyperfine` for startup benchmarks. On the reviewed ARM host, Go's race runtime rejects the VMA layout; use Linux amd64 CI for race checks. A writable `GOCACHE=/tmp/callm-audit-go-cache` works in the desktop sandbox.
