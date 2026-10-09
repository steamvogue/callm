# callm evaluation and upgrade suggestions

Date: 2026-10-09. Evaluated revision: `c27c9b8` (v0.9.0). Scope: report and reproducible audit evidence; no product changes or default changes.

Follow-up: [repair progress and validation](PROGRESS.md) records subsequent fixes. The findings and artifacts below describe the original evaluated revision.

**Recommendation: fix pipeline correctness and modernize the release toolchain first, then introduce model capability handling and validated output, followed by a small resumable pipeline runner.** The existing single-binary design is a good foundation. A general-purpose coding agent would be a much larger product than the useful “mini harness” described below.

The repository has 33 Go files, 5,643 lines including tests, and 3,239 production lines (97,495 bytes). It has thirteen presets, two protocol implementations, useful timeout controls, and no third-party Go modules. The strongest gaps concern whether a result is usable, how model capabilities are represented, and how repeated calls are tracked.

## Evidence and limitations

| Check | Result |
|---|---|
| Fresh `go test -count=1 -cover ./...` | Passed all five packages. Coverage: CLI 75.7%, client 79.9%, config 80.9%, exports 86.1%, UI 69.7%. |
| Existing local CLI audit | All 91 scenarios completed and its regression assertions passed. |
| New report probes | 18 cases reproduced the current behavior described below; these are observations, not tests asserting the desired future behavior. |
| Guarded-output example | ShellCheck and ten fake-provider fixture cases passed, including truncation/refusal rejection, literal-tag preservation, and preservation of an existing artifact on failure. |
| `go vet ./...`, actionlint, ShellCheck | Passed. |
| Root/chat/models/info/raw help | All five help commands exited successfully and displayed usage. Existing README/help consistency test also passed. |
| Native execution | Built and ran on Linux ARM64 with Go 1.26.0. |
| Public model catalog | Fetched OpenRouter's unauthenticated catalog: 469 entries; selected records retained. |
| Provider generation, billing, account access | Not tested; no billed provider calls made. |
| Race detector / other operating systems | Not run in this review. Existing project notes record an incompatible ARM VMA layout; the configured Linux amd64 CI race check was inspected, not executed remotely. |
| Vulnerability scan | No fresh vulnerability-database scan performed; toolchain findings below come from repository configuration and Go's release policy. |

The sandbox initially prohibited localhost listeners; the same tests passed when local sockets were permitted. This was an execution restriction, not a project defect. DuckDuckGo research was attempted first but returned a challenge, so current facts were checked through official documentation and the public catalog.

Evidence: [test output](/var/www/straitly/audit/2026-10-09/go-test.txt), [91 existing probes](/var/www/straitly/audit/2026-10-09/legacy-probes.json), [18 new probes](/var/www/straitly/audit/2026-10-09/probes.json), [probe implementation](/var/www/straitly/audit/2026-10-09/probe.py), [example checks](/var/www/straitly/audit/2026-10-09/example-checks.json), [help checks](/var/www/straitly/audit/2026-10-09/help-checks.json), [catalog snapshot](/var/www/straitly/audit/2026-10-09/openrouter-catalog.json). Empty lint logs in this directory record successful commands with no diagnostics.

## Confirmed problems and compatibility gaps

P1 means address before relying on unattended output or publishing upgraded defaults. P2 means important follow-up. Findings marked “documented behavior” are product choices worth improving, not undocumented regressions.

| Priority | Finding and evidence | Impact and suggested correction |
|---|---|---|
| P1 | **Truncated, empty, or refused results can appear successful.** Probes `truncated`, `stream-truncated`, `empty`, `refusal`, `filtered`, and `done-only` all exit 0. Anthropic `max_tokens` also exits 0. [Chat rendering](/var/www/straitly/cmd/callm/main.go:996) ignores finish reasons; [stream handling](/var/www/straitly/internal/client/client.go:216) accepts `[DONE]` without checking answer completion. | A job can publish partial text or an empty artifact. Preserve provider finish/stop/refusal information and distinguish transport success from usable completion. Reject malformed response shapes. Add a documented strict pipeline mode that fails on truncation, refusal, unsupported tool-only output, and unexpected empty answers; permit explicitly expected empty results. Do not retry refusals automatically. |
| P1 | **Literal `<think>` content is silently removed.** `Example: <think>literal text</think> end` becomes `Example:  end`; a JSON string containing those tags becomes an empty string. Reproduced in `literal-think` and `json-think`. [Renderer](/var/www/straitly/internal/ui/format.go:122). | Code, XML, quoted examples, and JSON values can be corrupted while the command succeeds. Make inline-tag interpretation explicit or model-capability-specific. Preserve literal content by default for structured output. Current workaround: use `--json` and extract the provider's content yourself. |
| P1 | **Output write errors are ignored.** Writing a completed answer to Linux `/dev/full` exits 0 (`stdout-write-failure`). [Renderer writes](/var/www/straitly/internal/ui/format.go:76) and [JSON output](/var/www/straitly/cmd/callm/main.go:1030) discard errors. | A failed output destination can look like a successful job. Return write/flush errors through the renderer and CLI; cover full-disk and failing-writer cases. Catalog JSON exports already check their stdout write. |
| P1 | **CI and releases build with unsupported Go 1.22.** Both [CI](/var/www/straitly/.github/workflows/ci.yml:20) and [release workflow](/var/www/straitly/.github/workflows/release.yml:24) pin it; `check-latest` only updates within that line. | Update release builds to patched Go 1.27.2 or newer supported patch, and test both supported Go lines. Go's official page lists 1.27.2 on October 8 and supports only the latest two major releases. Keep or raise the `go.mod` language minimum as a separate compatibility decision. This is not proof of a specific exploitable vulnerability. [Go release policy/history](https://go.dev/doc/devel/release). |
| P1 for model migration | **New Claude effort requests use the legacy schema.** `--ant -m claude-sonnet-5-5 --effort high` sends `thinking:{type:enabled,budget_tokens:4096}`. [Conversion](/var/www/straitly/internal/client/anthropic.go:202). | Official docs say Sonnet 5.5 rejects that manual-budget form. Introduce model-aware adaptive thinking and `output_config.effort` before changing the default. Local capture proves the request shape; rejection is documented, not live-tested. [Thinking compatibility](https://platform.claude.com/docs/en/build-with-claude/thinking), [effort configuration](https://platform.claude.com/docs/en/build-with-claude/thinking-steering-and-cost). |
| P2 | **Anthropic statistics undercount total processed input.** Synthetic usage of 10 uncached input + 100 cache-read + 20 cache-write + 5 output displays `15 tokens (10 in / 5 out)` in both output modes. [Usage conversion](/var/www/straitly/internal/client/anthropic.go:83). | Correct total is 135, with 130 input. Track cache reads/writes separately, include them in processed-input totals, and keep server-reported cost separate. Do not double-count cached tokens for providers whose input total already includes them. [Anthropic usage accounting](https://platform.claude.com/docs/en/build-with-claude/prompt-caching). |
| P2 | **Native Anthropic exports are labeled OpenAI-compatible.** All three editor export probes succeed but produce OpenAI provider configuration; [export code](/var/www/straitly/internal/export/export.go:325) has no protocol metadata. | Native `/messages` credentials/endpoint need a corresponding editor adapter. Add protocol-aware exporters or reject these combinations with clear guidance. Also preserve newer capability/output-limit fields. Export bytes were verified; imports were not tested in the editors. |
| P2 | **OpenAI normalization only recognizes o1/o3/o4.** A `gpt-6.1-sol` request keeps `max_tokens`; `gpt-6-luna --effort none` fails before HTTP. [Validation/normalization](/var/www/straitly/internal/client/client.go:92). | Introduce model-specific accepted efforts, sampling rules, and token-cap mapping. Current GPT-6 documentation allows `none` for Luna/Sol, but not Astra/6.1 Sol; sampling parameters are restricted with reasoning. Do not solve this with one permissive global enum. Prefer Responses for future tools. [OpenAI migration guide](https://developers.openai.com/api/docs/guides/latest-model). |
| P2, documented behavior | **Automatic provider detection only checks six presets.** With only `OPENAI_API_KEY`, bare `callm` asks for a Poolside key. [Selection](/var/www/straitly/cmd/callm/main.go:356). | Add an explicit persistent default-provider setting and a redacted `config explain`/`doctor`; improve the missing-key message. Keep explicit provider flags in automation. Changing the existing priority order silently could redirect workload and spending. |

Additional observations from code/document inspection:

- `--no-reasoning` controls display, not inference effort or cost. DeepSeek already documents this correctly; make the distinction prominent in pipeline examples.
- The live test script treats any stderr text as evidence of reasoning. A notice can satisfy that check, and modern models can legitimately omit thinking text. Replace it with assertions on the provider response/usage and a separately requested reasoning-display capability. This script was inspected; no paid run was performed.
- The agent skill's “Straitly gateway” example invokes bare `callm`, now a Poolside/default-detection call. `make info` also uses a DeepSeek gateway model without explicitly selecting that gateway. Make these examples explicit. [Skill](/var/www/straitly/skills/callm/SKILL.md:41), [Makefile](/var/www/straitly/Makefile:76).
- Local attachments are bounded individually at 64 MiB, but their combined prompt has no aggregate byte/token budget. Add a combined limit and an optional token preflight before supporting larger batches.
- Configuration still trusts the executable-parent `.env` by design, even with working-directory dotenv opt-out. Retain that caveat in installation examples; explicit pipeline settings are easier to reproduce.
- Keep the existing redirect isolation, per-provider credentials, EOF handling, size bounds, and strict SSE parsing. The new findings do not invalidate those repairs.

## Default-model decisions

These are **candidates for a measured upgrade**, not claims of superior quality on this project's workloads. Official model documentation establishes supported options; the unauthenticated OpenRouter catalog establishes listing, not account access or successful inference. Direct-provider IDs and gateway IDs must be checked independently.

| Preset | Current | Recommendation |
|---|---|---|
| OpenAI `--oa` | `gpt-4o` | Evaluate `gpt-6.1-sol` for general/code work and `gpt-6-luna` for extraction/classification. Prefer a balanced default plus an explicit fast profile; finish capability handling first. [6.1 Sol](https://developers.openai.com/api/docs/models/gpt-6.1-sol), [Luna](https://developers.openai.com/api/docs/models/gpt-6-luna). |
| Anthropic `--ant` | `claude-sonnet-4-6` | Target `claude-sonnet-5-5` after adaptive-thinking support. Evaluate `claude-haiku-5-5` for routine pipelines. Do not change only the string. [Current model IDs](https://platform.claude.com/docs/en/models/overview). |
| Claude shortcut on OpenRouter | `anthropic/claude-sonnet-4.6` | Candidate `anthropic/claude-sonnet-5.5`, confirmed in the saved public catalog. Validate effort translation through that gateway. Straitly and OrcaRouter require their own catalog checks. |
| OpenRouter `--or` | `deepseek/deepseek-v4-flash-0731` | Candidate `deepseek/deepseek-v4.1-flash`; both IDs remain listed in the fetched catalog, so the old default is not proven broken. Compare outputs before migration. [Public catalog](https://openrouter.ai/api/v1/models). |
| Straitly `--st` | `deepseek/deepseek-v4-flash-0731` | Verify its own authenticated catalog before choosing a replacement. Do not infer Straitly availability from OpenRouter. No newer Straitly default established here. |
| DeepSeek `--ds` | `deepseek-flash` | Keep: current official docs map this to V4.1 Flash. Add a first-class thinking on/off control and examples for non-thinking batch tasks. [Model details](https://api-docs.deepseek.com/quick_start/pricing/). |
| Poolside `--pool` and overall fallback | `poolside/laguna-s-2.1` | Keep pending workload evaluation. Poolside still presents Laguna S 2.1 and XS 2.1 as its latest models; XS is an evaluation candidate, but its exact API ID was not verified here. [Poolside docs](https://docs.poolside.ai/). |
| OrcaRouter `--orca` | `orcarouter/auto` | Keep the explicit auto-routing option. Recommend a pinned model for repeatable evaluations; no verified replacement for this provider was established. Record requested and returned model separately. |
| Moonshot `--ms` | `moonshot-v1-auto` | Evaluate `kimi-k3` and coding-specific `kimi-k2.7-code-highspeed`. Current docs support K3 efforts `low/high/max`, not the CLI's universal `low/medium/high`. Verify regional endpoint/key pairing; the preset uses China, while international examples use `api.moonshot.ai`. [Kimi API quickstart](https://platform.kimi.ai/docs/overview). |
| Kimi Code `--kimi` | `kimi-for-coding` | Keep compatibility alias; evaluate `k3-256k` for routine coding and `k3` for larger inputs where membership permits. The docs currently describe `kimi-for-coding` as K2.8 Preview, so the alias itself can change underneath a pipeline. Preserve real client identity. [Kimi Code models and regions](https://www.kimi.com/code/docs/en/). |
| ZAI `--zai` | `glm-4-flash` at `open.bigmodel.cn` | Add a separate international Z.AI endpoint/profile before considering `glm-5.3`. Official Z.AI examples use `api.z.ai`; do not repoint existing China credentials silently. Evaluate a cheaper model separately instead of assuming flagship is a cost-equivalent replacement. [Z.AI quickstart](https://docs.z.ai/guides/overview/quick-start). |
| Qwen `--qw` | `qwen-plus` | Retain until region-specific evaluation. Add optional `qwen3.8-flash` / `qwen3.7-plus` multimodal examples; verify availability and structured-output/thinking combinations for the selected region. [Alibaba model guidance](https://www.alibabacloud.com/help/en/model-studio/vision-model). |
| Groq `--groq` | `llama-3.3-70b-versatile` | Prioritize evaluating `openai/gpt-oss-120b` and `openai/gpt-oss-20b`; current Groq docs mark the existing Llama model Enterprise and list both GPT-OSS models as production options. Check access and output behavior first. [Groq catalog](https://console.groq.com/docs/models). |
| Ollama `--ollama` | `deepseek-r1` | Prefer an explicitly installed model and hardware-aware instructions over another universal default. Add local schema-validation examples. Ollama supports structured outputs, but callm does not yet expose JSON Schema. [Ollama structured output](https://docs.ollama.com/capabilities/structured-outputs). |

Recommended default policy: named profiles such as `fast`, `balanced`, and `code` should resolve to a visible provider/model configuration. Explicit flags should remain strongest. Show resolution without a network request, allow pinning, and never silently switch providers after an error.

## Features with the highest practical value

| Order | Proposal | Useful behavior and acceptance condition |
|---|---|---|
| 1 | Strict pipeline result + stable JSON envelope | Expose `status`, answer, refusal, finish reason, requested/returned model, usage, timings, and provider error category. Preserve existing raw `--json` behavior under its current name. Missing usage remains unknown. Truncated results cannot become successful pipeline artifacts. |
| 2 | JSON Schema + local validation | Proposed `--schema file.json` maps to provider-supported structured output and also validates the result locally. `--json-object` alone is not schema enforcement. Decide whether to add a maintained validator dependency or explicitly support a documented subset; do not claim full JSON Schema with a partial validator. |
| 3 | File-based request/prompt input | Proposed `raw --body-file file.json` / `--body-file -`, `--system-file`, and prompt templates. Existing raw only accepts a JSON positional argument, which is awkward for large bodies and escaping. Context `-f` adds filename/fence wrappers and is not a raw-body substitute. |
| 4 | Provider capability profiles | Separate protocol, account region, model family, effort values, thinking controls, token limits, output formats, and tool support. Keep unknown custom models usable with explicit settings. Refresh advisory catalogs separately from pinned runtime configuration. |
| 5 | Bounded retries and observable errors | Typed errors for authentication, rate limits, timeout, transport, incomplete response, and validation. Honor `Retry-After`; cap attempts and total deadline. Retry only configured eligible failures, avoid retrying after output has been exposed, and report ambiguous timeouts that may already have been billed. |
| 6 | `doctor` / configuration explanation | Offline redacted configuration, chosen provider/model, source of overrides, protocol, and local model check. Separate opt-in online catalog and generation checks so diagnosis does not unexpectedly incur generation charges. |
| 7 | Batch/JSONL jobs | Stable job IDs, bounded concurrency, one result/error per job, restart/resume, per-job timeout and token ceilings. Avoid one process startup per tiny record where a reusable HTTP client can help. Provider-native asynchronous Batch is a separate adapter with a separate lifecycle. |

Add machine-readable statistics before cost-based routing. Preserve uncached input, cached reads/writes, generated output, reasoning details where returned, and the provider's accounting semantics. Reasoning is often a subset of output; never add it twice. Label end-to-end completion tokens/second accurately; it is not a measurement of decode speed alone. Treat unknown cost as unknown, with optional clearly labeled estimates using a dated price catalog.

## A useful mini harness

Start with an explicit **prepare → call → validate → optional repair → save** workflow. It should handle repeatable transformations, extraction, reviews, and triage. Proposed `callm run pipeline.json` is future interface design, not a currently supported command.

Version 1 should have:

1. A versioned JSON manifest, named steps, prompt/system files, explicit provider/model, and input/output paths. JSON fits the existing dependency-free implementation.
2. A small fixed step vocabulary: deterministic preprocessing, model call, validation, and writing an artifact. Dependencies form an acyclic graph; start with sequential execution and optional bounded independent jobs.
3. Per-step deadlines, output ceilings, maximum attempts, a total token budget, and at most one explicitly configured repair call. Report budgets as best effort when usage is missing; do not promise a hard currency cap without provider enforcement.
4. A run directory with an append-only JSONL journal, atomic result files, manifest/prompt/model/input hashes, and timestamps. Resume only successful matching jobs; incomplete or changed jobs remain pending. Keep original provider JSON as an optional artifact.
5. Clear data retention: redact keys/authorization headers and make prompt/source logging explicit. Inputs and model output are data, not commands.
6. Deterministic validators first: JSON/schema checks, required fields, expected enum values, tests, or source-grounding checks. A model judge may supplement these later, with its own cost and disagreement reporting.

Do not add a model-controlled shell loop as the first harness feature. If tool use is later required, implement tool-call messages, tool result IDs, bounded loops, workspace controls, and cancellation deliberately. The current `Message`/`RespMsg` types do not represent tool conversations or preserve all reasoning/signature blocks. Reusing the text renderer as a conversation store would lose information.

For OpenAI, the current migration guide requires Responses for reasoning tool workflows on the newest models. This is a protocol adapter project, not just adding a `tools` flag. The existing `raw` command can help experiment with non-streaming JSON endpoints but does not supply tool orchestration, resumable state, or normalized Responses streaming. [OpenAI tool compatibility](https://developers.openai.com/api/docs/guides/latest-model).

## Pipeline examples to add

The following uses existing options. It demonstrates guarded output publication; its JSON filter is fixture-tested in this audit. Provider generation is not live-tested. Run from a working directory containing `input.txt`, with the intended OpenRouter credentials configured.

```bash
#!/usr/bin/env bash
set -euo pipefail
reply=$(mktemp)
staged=$(mktemp ./summary.json.tmp.XXXXXX)
trap 'rm -f "$reply" "$staged"' EXIT

callm --or -m deepseek/deepseek-v4.1-flash \
  --no-stdin --json --json-object --timeout 90s --max-tokens 4096 \
  -f input.txt 'Return a JSON object with a nonempty string field named summary.' \
  > "$reply"

jq -e '
  if .choices[0].finish_reason != "stop" then error("incomplete response") else . end
  | .choices[0].message
  | if (.refusal // "") != "" then error("refusal") else .content end
  | fromjson
  | if type != "object" then error("expected object") else . end
  | if (.summary | type) != "string" then error("expected summary") else . end
  | if (.summary | length) == 0 then error("empty summary") else . end
' "$reply" > "$staged"

mv -- "$staged" summary.json
```

This preserves literal `<think>` strings by reading the original JSON envelope and publishes only after validation. It deliberately rejects non-`stop` completions. Native Anthropic needs a different envelope/stop-reason extractor. Until write-error handling is fixed, this pattern improves detection but cannot guarantee every disk failure is detected by callm itself.

Other high-value examples:

| Example | What it teaches |
|---|---|
| CI failure triage | Capture test logs to a file, preserve the test exit status, then ask callm for a summary. The summarizer must never turn a failing build green. |
| Changed-file review | Generate a bounded Git diff, exclude secrets/generated files, run one review call, and retain the source commit and prompt hash. |
| CSV classification | Use qsv to select, deduplicate, and batch records before inference; retain stable row IDs and validate an exact one-to-one output mapping. |
| Documentation extraction | Convert local HTML using `htmlmd --profile plain-text --extract-selector main` before supplying it as context; retain URL/date provenance when fetching public documents. |
| Local structured extraction | Discover an installed Ollama model, use a supported schema path once implemented, and compare results against a small labeled fixture set. |
| Scheduled model-catalog comparison | Save JSON catalogs, compare IDs/capabilities with jq, and flag missing defaults. A catalog change should trigger review rather than silently changing runtime models. |
| Cheap-first escalation | Use a smaller model for schema-bound jobs; retry with an explicitly selected stronger model only on validation failure. Track both calls and keep provider changes visible. |

## Known tools and newer services worth using

| Tool/service | Recommendation and verification |
|---|---|
| `jq` | Already installed. Validate/extract response JSON, join job IDs, and compare catalogs. Used in this audit. |
| `htmlmd` | Installed at `/home/lordtime/.local/bin/htmlmd`; a local HTML extraction fixture passed. Reduce navigation/markup before paying to process page content. |
| `qsv` | Host-native 21.1.0 confirmed. Useful for deterministic CSV preprocessing; no need for an LLM to count, select, or deduplicate rows. Keep the host-built binary specified in owner tooling notes. |
| `ast-grep` | 0.45.0 confirmed via the existing `sg` alias; it now warns to prefer the canonical `ast-grep` command. Use structural code extraction to reduce review context. |
| actionlint / ShellCheck | Both ran successfully. Add them to CI; currently they are local maintenance checks, not workflow steps. |
| `govulncheck` | Not on PATH in this review. Add as a pinned CI tool after the compiler upgrade, including standard-library exposure. No “zero vulnerabilities” claim is made here. [Official Go guidance](https://go.dev/doc/security/vuln/). |
| Promptfoo | Consider as an optional development dependency for prompt/model comparisons. Its `exec:` provider can wrap callm and use stdout as the answer. Return answer text from the wrapper; a JSON-looking stdout is still treated as a string, not normalized usage metadata. Not installed or run here. [Custom-script provider](https://www.promptfoo.dev/docs/providers/custom-script/). |
| OpenAI Decisions API | Newly documented beta for typed predicates, category choices, and rubric scores with `gpt-6-luna`. A worthwhile isolated experiment for routing/classification, not a replacement for free-form chat or schema extraction. `raw /decisions` is the existing experimentation path; native support would be optional. Provider speed claims were not benchmarked here. [Decisions documentation](https://developers.openai.com/api/docs/guides/decisions). |
| Existing coding-agent tools | Prefer adapters to an existing agent when the task requires arbitrary repository edits or browser work. Keep callm focused on a small, scriptable inference interface; no additional agents were launched for this report. |

## Evaluation plan before promoting defaults

Build a versioned set of 24 representative cases: six structured extractions, six code/review tasks, six log/document summaries, and six edge cases involving malformed output, refusal, truncation, and long inputs. Keep deterministic transport fault tests separate from quality evaluation.

For each provider being changed, compare the old default with up to two candidates, twice per case: up to 144 generation calls per provider. This is a proposed experiment, not a completed benchmark. Start with a smaller representative batch to measure actual usage before approving the full run.

Measure schema/validator pass rate, task correctness, unsupported-setting failures, truncation/refusal rate, total latency, input including cache categories, generated output including reasoning where accounted for, and reported cost. Report distributions and failures, not just mean speed. Record exact model IDs, returned model, settings, prompt hashes, and run date. If model judging is added, count its calls and tokens separately.

Promote a candidate only after it passes request compatibility tests and the workload's agreed quality/cost/latency thresholds. Catalog presence and a successful minimal response are insufficient. External account access, rate limits, and paid evaluation are dependencies separate from agent implementation effort; no token-cost or throughput estimate is fabricated here.

## Suggested implementation sequence

1. **Correctness release:** finish/refusal/empty-result semantics, write errors, literal-content preservation, accurate usage accounting, current Go builds, and focused regressions.
2. **Compatibility release:** capability-aware reasoning/token parameters, protocol-aware exports, explicit default configuration, and synchronized examples. Evaluate and change provider defaults individually.
3. **Pipeline release:** stable result JSON, schemas/validation, body/prompt files, typed errors and bounded retry policy, then examples and evaluation fixtures.
4. **Harness release:** versioned manifest, sequential steps, journal/resume, bounded batches, and budget visibility. Add tool conversations or provider-native Batch only when a concrete workflow needs them.

Every product change should update CLI/subcommand help, README, examples, changelog, repository callm skill, and installed skill copies under the project's existing maintenance rule. Preserve the README help synchronization test. This report leaves those product changes for the implementation phase.
