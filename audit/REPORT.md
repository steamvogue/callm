**Audit findings resolved — 2026-09-05**

All 19 reported code findings are addressed, including the redirect and pagination findings added during the deeper review. The 300-second default remains in place. Unsupported provider/flag combinations now produce explicit errors; this does not imply every provider supports every option.

| Finding | Resolution | Regression evidence |
|---|---|---|
| 1. Cross-provider key fallback | Only explicit/global and selected-provider credentials are resolved | `TestKeyIsolationAndPrecedence` |
| 2. Unbounded HTTP / Anthropic stop | 300-second default, configurable total/header/idle limits; `message_stop` terminates immediately | Timeout tests; `TestAnthropicStopAndIdleTimeout` |
| 3. Lost or blocking stdin | Portable EOF reader with context cancellation, input timeout, and `--no-stdin`; obsolete OS polling removed | `TestStdinWaitAndDeadline`; CLI delayed-input/cancellation assertions |
| 4. Malformed / truncated SSE | MIME checking, full event framing, optional space, CR/LF and multiline data, strict JSON and completion markers | `TestStreamValidation` |
| 5. False-success HTTP/API errors | Raw and chat requests reject error statuses and API error envelopes | `TestStatusErrorsAndRawPreservation` |
| 6. Retired Claude default | Direct default is `claude-sonnet-4-6`; gateway shortcut is `anthropic/claude-sonnet-4.6` | `TestCLIAnthropicProxy`; official lifecycle/catalog checks from baseline |
| 7. Anthropic options and images | Image blocks are converted, top-p retained, unsupported JSON-object requests explicitly rejected | `TestAnthropicOptions`; CLI image/top-p assertions |
| 8. Explicit token cap overridden | Invalid thinking-budget/cap combinations fail; only an unspecified default cap may be adjusted | `TestAnthropicOptions` |
| 9. Non-streaming reasoning controls | Both output modes use the same reasoning renderer, including `thought` and inline tags | `TestCLIOutputAndValidation`; CLI reasoning assertions |
| 10. Lossy JSON and missing stats | Preserve original provider response bytes and print requested stats independently | `TestStatusErrorsAndRawPreservation`; `TestCLIOutputAndValidation` |
| 11. Missing usage / string cost | Request stream usage when stats are enabled; show unavailable counts; parse finite string costs | `TestUnknownMetadataAndStringCost`; CLI usage assertions |
| 12. Provider-before-command syntax | Parse leading options before dispatch without scanning their values as command names | `TestGlobalSubcommandDispatch`; CLI model-list assertion |
| 13. Ignored URL/client overrides | Shared URL precedence and reuse of configured HTTP client for all paths | `TestBaseURLPrecedence`; timeout tests |
| 14. Protocol inferred from URL substrings | Explicit provider metadata survives custom endpoints; optional inference uses exact hostnames only | `TestProviderSelectionAndPagination`; `TestCLIAnthropicProxy` |
| 15. Invalid/conflicting values | Full numeric parsing, range validation, explicit conflict errors, provider-specific normalization | `TestRequestValidationAndGatewayNormalization`; CLI rejection assertions |
| 16. Raw parse errors ignored | Abort before HTTP requests on parse errors; invalid raw JSON also rejected | CLI raw-unknown-flag assertion; timeout CLI tests |
| 17. Unknown model prices shown as zero | Missing/invalid prices and context display as unknown; reported zero prices remain zero | `TestUnknownMetadataAndStringCost` |
| 18. Credentials follow cross-origin redirects | Reject cross-origin and downgrade redirects before contacting the destination | `TestRedirectCredentials` |
| 19. Anthropic catalog pagination ignored | Follow `last_id`/`after_id` under one operation deadline; detect repeated cursors | `TestProviderSelectionAndPagination` |

Additional audit observations are addressed: explicit `--stream=false` works; streaming defaults to terminal stdout; conflicting output/provider switches are rejected; raw pretty-printing retains large JSON numbers; local input/response/event sizes are bounded. The live test script asserts outputs, includes Anthropic, uses explicit provider keys, and exits 2 if all providers are skipped. The misleading race-test Makefile description is corrected, `make test-race` is available, and CI enables race detection.

Timeout usage:

```bash
callm --timeout 600 "prompt"
callm --timeout 10m --header-timeout 60s --idle-timeout 120s "prompt"
callm --stdin-timeout 30s "summarize the pipe"
callm --no-stdin "ignore an inherited pipe"
```

`--timeout` defaults to 300 seconds. Unspecified header and stream-idle limits inherit that value. Explicit phase overrides take precedence. `--timeout 0` disables the overall limit and inherited header/idle limits; `--stdin-timeout` remains independent. All API commands support the total/header limits; idle/input controls apply to chat.

Validation:

- Local Go unit/integration tests and `go vet` pass: [results](verification-fixed.txt).
- The [CLI regression harness](probe_cli.py) executes 91 local scenarios and asserts the repaired behaviors: [captured requests and outputs](probe-results-fixed.json).
- Linux amd64/arm64, macOS amd64/arm64, and Windows amd64 compile checks: [results](cross-build-fixed.txt). Native macOS/Windows execution was not performed.
- `actionlint`, ShellCheck, and `git diff --check` pass. A fake executable verifies live-script success, incorrect-marker failure, and all-skipped exit 2 without paid provider calls.
- Local race execution remains unavailable because ThreadSanitizer rejects this ARM host's VMA layout. The code change enables it in Linux amd64 CI; CI itself was not run remotely.
- Provider generation quality, live billing, and account-specific availability have not been tested. Current provider documentation supports the selected Claude 4.6 model and manual thinking configuration, and the public OpenRouter catalog lists the gateway shortcut.

Reproduce from the project root:

```bash
export GOCACHE=/tmp/callm-audit-go-cache
go test -timeout=30s ./...
go vet ./...
python3 audit/probe_cli.py > /tmp/callm-fixed-probes.json
actionlint
shellcheck scripts/test_live.sh
```

Normal regression tests are in [CLI tests](../cmd/callm/regression_test.go), [client tests](../internal/client/regression_test.go), [configuration tests](../internal/config/config_test.go), and [metadata tests](../internal/ui/stats_test.go). The earlier overlay probe and pre-fix result files are historical evidence and are superseded by these tests; do not use their old defect assertions against the repaired implementation. The [historical audit](BASELINE.md) retains the original findings and source documentation.

Reusable host tools: Go `/usr/local/go/bin/go`, `rg` on PATH, actionlint `/home/lordtime/go/bin/actionlint`, ShellCheck `/usr/bin/shellcheck`, hyperfine `/usr/bin/hyperfine`, and document reader `/var/www/se/DuckDuckGo/bin/ddg-search`.

Release documentation also covers every registered chat alias, all provider defaults and credential aliases, phase-timeout inheritance, configuration precedence, and subcommand usage. `TestREADMEHelpReference` prevents the README CLI reference from drifting from executable help. The agent skill is maintained alongside product changes; see [maintenance instructions](../AGENTS.md).
