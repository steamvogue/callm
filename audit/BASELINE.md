**Historical audit — findings below describe the pre-fix implementation. See [current status](REPORT.md).**

**Project evaluation — 2026-09-05**

Reviewed commit `2307a55ab75c50112d08e0c11a162d01a20d7e5a` using Go 1.26.0 on Linux/ARM64, then implemented the requested timeout change. Verdict: the small, dependency-free architecture and basic request/rendering paths are effective, but input reliability, provider conversion, and option combinations still do not satisfy an “all knobs working” requirement.

**Implemented and validated:** all API requests now default to **300 seconds**, including streaming, Anthropic, model lookup, and raw requests. `--timeout 600`, `--timeout 10m`, and `--timeout 0` respectively select seconds, a duration, and no overall request limit. Every command reuses its configured HTTP client. Invalid timeout values abort before HTTP traffic; fixing this also resolves the raw-command parse-error finding. The timeout includes response-body reading but begins after local stdin/files are consumed. No separate stream-idle or stdin timeout has been added.

The new tests verify actual default deadlines on all six client paths, short timeouts against stalled headers and bodies, and CLI propagation/rejection. `go test -timeout=30s -coverprofile=… ./...` and `go vet ./...` pass. Current coverage is 55.1% overall (CLI 59.1%, client 70.1%, UI 39.2%, config 0%). See [verification-after-timeout.txt](verification-after-timeout.txt) and [internal-probe-results-after-timeout.txt](internal-probe-results-after-timeout.txt). Race detection remains unavailable on this host. Baseline cross-compilation succeeded for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64; those were compile checks, not native execution.

Baseline evidence comprises the existing tests, 85 local CLI scenarios in [probe-results.json](probe-results.json), and four internal probes in [internal-probe-results.txt](internal-probe-results.txt). The expanded internal probes add redirect and pagination checks. CLI probes capture outgoing requests and observed outputs; their successful execution does **not** mean each behavior is correct. All generation requests used dummy credentials and local mocks or an in-process transport. Actual generation quality, billing, and provider acceptance beyond the cited documentation remain unverified. The public, unauthenticated OpenRouter catalog was also read: it lists the default DeepSeek model, but does not list the hardcoded `anthropic/claude-3.7-sonnet` shortcut. Catalog presence does not prove that a generation will succeed. [OpenRouter model catalog](https://openrouter.ai/api/v1/models).

**Additional findings from the deeper review:**

- **P1 — Anthropic credentials follow redirects to a different host.** The default HTTP redirect behavior retains `x-api-key`. A local 307 redirect from `127.0.0.1` to `localhost` delivered the original dummy Anthropic secret to the second server. The same client setup is used by streaming, chat, catalog, and raw paths. Configure a redirect policy that rejects untrusted cross-origin redirects or strips credentials. This is distinct from provider-key fallback below and remains unfixed. Evidence: `TestAuditAnthropicRedirect` in the expanded internal results; relevant setup is [client.go:29](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L29).
- **P2 — Anthropic catalog pagination is ignored.** A response with `has_more: true` triggered only one request, so `models` and `info` can omit available models beyond the first page. `ModelListResponse` lacks pagination fields and `ListModels` never follows cursors. Anthropic documents a default page size of 20 with `after_id`, `has_more`, and `last_id`. Implement pagination or retrieve an individual model for `info`. [Anthropic List Models reference](https://platform.claude.com/docs/en/api/models/list).

**Baseline findings, ordered by priority.** P1 means high operational impact; P2 means a functional defect that should be corrected. The overall timeout portion of finding 2, the HTTP-client portion of finding 13, and all of finding 16 are now resolved. Remaining findings are unfixed. The following original behavior is retained as the before-change evidence, with source links into the reviewed commit; consult the current status above for timeout behavior.

1. **P1 — Missing provider credentials can send an unrelated provider's secret.** [config.go:198](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/config/config.go#L198) scans keys for every provider when the selected provider has no key. The internal probe selected OpenAI with only a dummy DeepSeek key and obtained that DeepSeek key without an error. The HTTP layer would send it to the selected endpoint. Resolve only the explicit/global override and selected provider's keys; report a missing credential instead of crossing provider boundaries.

2. **P1 — Streaming and Anthropic non-streaming requests can wait indefinitely.** [client.go:32](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L32) disables the client timeout, while [main.go:141](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L141) creates a signal context without a deadline. [anthropic.go:190](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/anthropic.go#L190) uses that same unbounded client for non-streaming requests. Transport probes directly confirmed absent deadlines. Stalled-server probes stayed alive until signaled; they do not purport to prove infinity by elapsed time. Add configurable request, response-header, and stream-idle limits. Also terminate Anthropic streams on `message_stop`: an already-completed message with a connection kept open currently waits until the caller's context expires.

3. **P1 — Stdin can be silently dropped or block cancellation.** [main.go:832](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L832) tests pipe readiness only once with a zero timeout. A producer delayed by 150 ms resulted in a request containing only `summarize`, omitting its context. Conversely, a pipe containing initial bytes but kept open blocks in `io.ReadAll`; SIGTERM did not release the probe and it required a kill. [stdin_other.go:5](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/stdin_other.go#L5) always returns false, so piped stdin is skipped on Windows. Read intentional piped input to EOF with cancellation and an explicit input-wait policy; propagate read errors currently ignored at line 580.

4. **P1 — Corrupt or incomplete streams can be reported as success.** [client.go:111](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L111) silently skips malformed JSON and accepts EOF without a completion marker. Malformed-only, early-EOF, and ordinary-JSON-in-stream-mode probes all exited 0, sometimes with no output. Both streaming parsers also require `data: ` and discard valid `data:` events without the space. Parse SSE events, validate the response media type and completion state, and return errors for corruption or interrupted completion. The space is optional under the [WHATWG SSE specification](https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation).

5. **P1 — HTTP/API failures are not consistently reflected in exit status.** [client.go:270](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L270) returns raw bodies without checking status: a mocked HTTP 429 exited 0. [client.go:194](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L194) decodes `ChatResponse.Error` but does not reject it; a 200 response containing an error produced empty output and exit 0. Check status and API error envelopes consistently so automation can detect failure.

6. **P1 — The Anthropic preset defaults to a retired model.** [config.go:41](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/config/config.go#L41) and [main.go:574](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L574) select `claude-3-7-sonnet-20250219`. Anthropic lists its retirement as February 19, 2026 and states retired-model requests fail. Replace the direct-API default with a supported model and update examples after validating its parameter compatibility. This finding concerns Anthropic direct; gateway availability was not tested. [Anthropic model lifecycle documentation](https://platform.claude.com/docs/en/about-claude/model-deprecations).

7. **P2 — Anthropic conversion loses options and sends the wrong image schema.** [anthropic.go:248](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/anthropic.go#L248) forwards OpenAI `image_url` blocks unchanged; Anthropic requires `image` blocks with a `source`. The outgoing mock request also lacked the supplied `--top-p` and `--json-object` settings because `anthropicReq` has no corresponding mapping. Convert supported options and reject unsupported combinations explicitly rather than silently dropping them. [Anthropic image request examples](https://platform.claude.com/docs/en/build-with-claude/vision).

8. **P2 — Thinking can override an explicit output-token cap.** [anthropic.go:291](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/anthropic.go#L291) raises `max_tokens` when it is below the thinking budget. `-n 100 --thinking-budget 1024` sent `max_tokens: 3072`. Default-budget adjustment is reasonable when no cap was specified, but an explicit cap should be honored or rejected with an explanation.

9. **P2 — Non-streaming reasoning controls differ from streaming.** [main.go:763](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L763) does not use the inline-thinking parser and omits the `thought` fallback. `--no-stream --no-reasoning` printed `<think>private</think>answer`; `--no-stream --only-reasoning` returned nothing for inline thinking or the `thought` field. Use the same reasoning normalization for both paths.

10. **P2 — `--json` is lossy and disables `--stats`.** [main.go:757](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L757) re-serializes a restricted response struct and returns before stats. A provider extension supplied by the server vanished, and `--json --stats` emitted no stats. Unknown metadata, tool-call fields, and other unsupported fields cannot survive this path. Retain raw response bytes for the documented full/unparsed mode and print requested stats separately.

11. **P2 — Streaming statistics can display misleading zeros; string costs disappear.** [types.go:40](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/types.go#L40) cannot express `stream_options.include_usage`. The opt-in usage mock therefore returned no usage, and `--stats` reported zero tokens for a nonempty answer. OpenAI documents that opt-in for the aggregate usage chunk. Separately, [types.go:70](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/types.go#L70) ignores string costs despite declaring them supported: `"0.125"` produced no cost display. Request usage where supported, distinguish unavailable counts from zero, and parse valid string costs. [OpenAI Chat Completions reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create).

12. **P2 — Documented provider-before-subcommand syntax sends a chat request.** [main.go:159](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L159) dispatches solely on the first argument. The documented `callm --oa models gpt` becomes a chat prompt `models gpt`, potentially incurring generation charges. `callm models --oa gpt` works. Parse global options before command dispatch or align all examples with the actual grammar.

13. **P2 — Configuration overrides are inconsistent between paths.** [main.go:345](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L345) ignores `OPENAI_BASE_URL` and `STRAITLY_BASE_URL` for `info`, although chat/models support them. The internal probe set `OPENAI_BASE_URL` and observed an attempted default OpenAI URL through a mock transport. [client.go:173](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L173), lines 216 and 264 create new HTTP clients, bypassing `Client.HTTPClient` for Chat, ListModels and RawRequest. The configured 1 ms client and custom transport were never used. Centralize endpoint resolution and reuse the configured HTTP client/transport with request-specific deadlines.

14. **P2 — Protocol choice depends on URL substrings rather than the selected provider.** [anthropic.go:61](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/anthropic.go#L61) only selects Anthropic conversion when the URL contains `api.anthropic.com`. The `--ant` preset pointed at a normal local custom URL sent `/chat/completions` with bearer authentication. Inserting that domain string into the local URL path switched to `/messages` and `x-api-key`. [client.go:51](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L51) similarly ties gateway reasoning conversion to string matching. Keep API protocol/provider metadata explicit so custom proxy URLs preserve the selected behavior.

15. **P2 — Invalid and conflicting parameter values can be silently accepted.** [main.go:469](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L469) uses scanning that accepts numeric prefixes: `--max-tokens 123junk` became 123 and `--temp 0.7junk` became 0.7. Negative token counts, `--top-p 2`, and `--effort banana` also reached the server. [client.go:43](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/client/client.go#L43) leaves both token-limit fields populated for an o3 model when both flags are supplied, despite `max_tokens` being unsupported for o-series. Gateway `--effort` also wins over `--thinking-budget` in the normalized reasoning object without reporting the conflict. Validate complete numeric strings and provider-specific ranges; define or reject incompatible combinations. [OpenAI parameter reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create).

16. **P2 — Raw parse errors do not stop the request.** [main.go:395](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/cmd/callm/main.go#L395) discards `FlagSet.Parse` errors. `raw --bogus /endpoint '{}'` printed a flag error, still posted the JSON, and exited 0. Abort on parse failure before creating any request.

17. **P2 — Missing model metadata is presented as zero price.** [table.go:52](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/ui/table.go#L52) and [table.go:80](https://github.com/steamvogue/callm/blob/2307a55ab75c50112d08e0c11a162d01a20d7e5a/internal/ui/table.go#L80) initialize unavailable pricing to `0`. A model response containing only an ID was displayed as `$0/Mtok` input/output and zero context. Use “unknown” or “not provided”; absent price metadata does not establish that a model is free.

**Baseline timeout assessment (superseded by the 300-second implementation above).** No CLI timeout, idle timeout, response-header timeout, retry-count, or backoff knobs existed. Transport inspection verified the following; fixed deadlines were inspected rather than waiting 30/120 seconds for each to expire.

| Operation | Current limit | Assessment |
|---|---|---|
| OpenAI-compatible streaming | No request deadline or stream-idle limit | Unsatisfactory for unattended execution |
| Anthropic streaming | No deadline; does not stop on `message_stop` | Unsatisfactory |
| Anthropic non-streaming | No deadline | Unsatisfactory |
| Other non-streaming chat | Fixed 120 seconds overall | Bounded, but cannot be tuned for long reasoning or slower local models |
| `models` / `info` | Fixed 30 seconds overall | Reasonable default, but override path is missing |
| `raw` | Fixed 120 seconds overall, buffers entire response | Bounded but unsuitable for indefinite raw streams |
| Piped stdin | Instant readiness test, then unbounded read | Can drop input or stall before HTTP begins |

A suitable design would expose an overall request timeout and separate header/idle limits, with documented defaults and an explicit opt-out for intentionally long operations. Propagate cancellation through input reads. If retry support is added, bound attempts and elapsed time, honor applicable rate-limit responses, and avoid replaying an already-partially-delivered generation automatically.

**Knob coverage.** “Works” below means local parsing and payload/output behavior were verified, not that every provider supports the option. This is the baseline matrix with the added timeout knob; raw parse-error behavior is now corrected as noted above.

| Control | Observed behavior |
|---|---|
| `--timeout` (new) | Default 300 seconds on all commands; seconds/duration override supported; zero disables; malformed/negative values rejected before HTTP |
| All provider flags and aliases | Accepted and select configured model IDs; default-model availability generally unverified; Anthropic/default and custom-protocol defects above |
| `--claude` | Selects hardcoded Claude model; affected by retired direct default |
| `-m`, `--model` | Works; explicit model takes precedence in source |
| `-k`, `--key`, `--api-key` | Works with explicit dummy keys |
| `--key-env`, `--api-key-env` | Works; missing explicit env rejected; unsafe cross-provider fallback above |
| `--api`, `--base-url` | Works for URL override; provider protocol conversion is URL-dependent |
| `-s`, `--system` | Works; Anthropic system extraction covered by existing test |
| `-t`, `--temp`, `--temperature` | Generic payload works, including zero; invalid suffixes accepted; o1/o3 temperature deliberately removed |
| `-n`, `--max-tokens` | Generic payload works; cap violation and o-series conflict above |
| `--max-completion-tokens` | Serialized; dual-limit handling defective |
| `--effort`, `--reasoning-effort` | Serialized/translated; allowed values and combinations not validated |
| `--thinking-budget` | Serialized/translated; explicit cap and effort conflicts above |
| `--top-p` | Generic payload works, including zero; dropped by Anthropic conversion |
| `-f`, `--file`, repeated files | Works; both file contents observed in prompt |
| `--image`, repeated local/URL images | Generic image blocks and local base64 work; Anthropic schema defective |
| Stdin | Ready input worked; delayed/open-pipe input defective; Windows pipe path unsupported by implementation |
| `--json-object` | Generic request includes response format; Anthropic drops it |
| `--stream`, `--no-stream` | Basic switches work; redirected stdout still streams by default despite documentation implying a terminal-dependent default |
| `--stream=false` | Still streams; an explicitly false flag cannot disable the hardcoded default |
| `--stream --no-stream` | No-stream always wins regardless of ordering; conflict not rejected |
| `--reasoning`, `--no-reasoning`, `--only-reasoning` | Streaming examples work; non-streaming defects above; only-reasoning output goes to stderr |
| `--only-reasoning --no-reasoning` | Suppresses both channels, exits successfully; conflict not rejected |
| `--stats` | Non-streaming numeric usage/cost works; stream usage, string cost, JSON combination defective |
| `--json` | Valid JSON output, but not full/raw response preservation |
| `models [FILTER]`, `info MODEL` | Basic mocked catalog works; command placement, endpoint override, and missing metadata defects above |
| `raw ENDPOINT JSON` | Posts payload and prints response; status and parse-error defects above |
| `version`, `-v`, `--version`, help | Work without API traffic |
| `CALLM_BASE_URL`, `CALLM_MODEL` | Resolution implemented; global URL precedence directly probed |
| `STRAITLY_BASE_URL`, `OPENAI_BASE_URL` | Chat/models implemented; info ignores them |
| `STRAITLY_MODEL`, `OPENAI_MODEL` | Scoped to matching preset in code; not separately exercised end-to-end |
| `.env`, config files | Quoted/export-style local `.env` loading and preservation of nonempty existing values verified; executable-parent and home-file discovery reviewed only |

**Baseline validation and effectiveness.** Existing `go test ./...` and `go vet ./...` pass. `actionlint` and `shellcheck scripts/test_live.sh` pass. Baseline statement coverage was 15.0% overall: client 26.4%, UI 39.2%, CLI/config 0%. The provider-named tests mostly exercise the same permissive mock with different model strings; they do not verify actual provider contracts. The original Anthropic test only exercised request conversion, not HTTP/SSE execution. The new timeout tests exercise both HTTP modes.

`go test -race -cover ./...` cannot execute on this host: ThreadSanitizer reports `unsupported VMA range` / `Found 47 - Supported 48`. This is an environment limitation, not a detected race. The Makefile describes its test target as using race detection, but invokes `go test -v` without `-race`, as does CI. Go 1.22 compatibility and native macOS/Windows execution were not tested.

The live-test script reports “All Live API Tests Completed Successfully” even with all providers skipped; this was reproduced with an empty environment in [live-no-keys-result.txt](live-no-keys-result.txt). It does not assert the requested markers or reasoning output, and explicitly excludes Anthropic. It therefore cannot certify provider coverage.

`--version` startup measured **2.7 ms mean**, approximately **2.3–4.1 ms** over 30 warm runs without a shell using hyperfine. See [startup.json](startup.json). This supports low startup overhead on this machine; it is not a cold-cache benchmark, generation-latency measurement, or proof of a universal sub-3ms bound. Streaming avoids accumulating the whole answer, but non-stream responses and input files are read without size limits. No automatic retries or backoff are implemented.

**Reproduce and reuse.** Run from `/var/www/straitly`:

```bash
export GOCACHE=/tmp/callm-audit-go-cache
python3 audit/probe_cli.py > audit/probe-results-after-timeout.json
python3 - <<'PY'
import json
from pathlib import Path
root = Path.cwd()
Path('/tmp/callm-audit-overlay.json').write_text(json.dumps({'Replace': {
    str(root / 'cmd/callm/audit_internal_test.go'):
    str(root / 'audit/probe_internal_test.go.txt')
}}))
PY
go test -v -overlay=/tmp/callm-audit-overlay.json ./cmd/callm -run TestAudit
```

The Go overlay exposes audit probes without editing application files or adding them to the normal test suite. These probes include assertions about the observed defects; update their expectations when fixing the implementation. Verified reusable host tools: Go `/usr/local/go/bin/go`, `rg` on PATH, actionlint `/home/lordtime/go/bin/actionlint`, ShellCheck `/usr/bin/shellcheck`, hyperfine `/usr/bin/hyperfine`, and public-document reader `/var/www/se/DuckDuckGo/bin/ddg-search`.
