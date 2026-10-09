# Repair progress and observations

Date: 2026-10-09. Starting revision: `c27c9b8` (v0.9.0).
Source: [evaluation report](REPORT.md). The report and its baseline artifacts remain historical evidence.

## Scope and sequence

Fix confirmed behavior and compatibility problems individually, with focused mock regressions and documentation updates. Keep existing provider/model defaults pending workload evaluation. No paid provider generation is needed for these repairs.

| ID | Work item | State |
|---|---|---|
| B1 | Completion/refusal/empty-response validation, including native Anthropic streams | Fixed; focused mocks passed |
| B2 | Preserve literal inline thinking tags; make parsing explicit | Fixed; focused mocks passed |
| B3 | Propagate output and statistics write errors | Fixed; focused mocks passed |
| B4 | Count Anthropic uncached/cache-read/cache-write input correctly | Fixed; focused mocks passed |
| B5 | Move CI/release builds to supported patched Go lines | Fixed; both patched compiler suites passed |
| B6 | Model-aware native Claude adaptive thinking and effort | Fixed; focused regressions passed |
| B7 | Prevent native Anthropic catalogs from producing incompatible editor configurations | Fixed; focused regressions passed |
| B8 | Model-aware OpenAI reasoning efforts, sampling, and token caps | Fixed; focused regressions passed |
| B9 | Explain missing preset keys without silently changing provider selection | Fixed; focused regressions passed |
| O1 | Correct explicit-provider examples and live-test reasoning assertions | Fixed; fixtures/lint passed |

## Validation rules

- Record commands and observed outcomes; compilation, mock execution, and live provider execution are different evidence.
- Run focused regressions after each repair; run the full tests, vet, behavioral probes, help synchronization and relevant linters at completion.
- Preserve the 18 original diagnostic observations. Add after-repair evidence rather than rewriting baseline results.
- Synchronize README/help, release notes, repository skill and installed skill copies. Document any environment limitation.
- Race checks require a supported host; repository notes identify an ARM VMA limitation here. CI configuration is not evidence of a completed remote run.

## Decisions and remaining observations

- Automatic provider priority is documented behavior. Improve the diagnostic in B9; persistent defaults/config explanation remain a follow-up feature.
- Rejecting editor exports for an unsupported protocol is a valid bounded correction; a working native adapter needs editor-specific validation.
- Combined attachment limits, schema validation, result envelopes, retries, evaluation fixtures and the mini harness remain follow-up features.
- Record any additional defect discovered while fixing these items before changing its behavior.

## Work entries

Implementation and measured validation entries follow for each item.

### B1 — Completion semantics

- Added retained refusal/tool/finish metadata and completion validation. Native Anthropic message_delta stop reasons now reach the CLI. Missing response arrays are malformed.
- Buffered invalid text is rejected before stdout; streamed partial output can already exist when the final reason fails. Stats remain available on completion failures. --allow-empty permits explicitly expected empty output.
- Compatibility decision: --json alone is a transport/diagnostic envelope; --strict opts it into result validation. Nonempty legacy responses without finish reasons remain accepted outside strict mode. No automatic refusal retries.
- Validation: `GOCACHE=/tmp/callm-audit-go-cache go test ./cmd/callm ./internal/client -run 'TestCompletionExitStatus|TestStreamingCompletionExitStatus|TestREADMEHelpReference' -count=1` passed. CLI tests exercised 14 buffered cases and 5 streamed cases, including Anthropic truncation/tools. Client package had no matching tests in this focused invocation.
- Documentation: root/chat help, README reference/behavior, changelog and repository skill updated; installed-copy synchronization is a final step.

### B2 — Literal content

- Inline tags are preserved by default; --parse-think makes the existing parser explicit. Direct reasoning fields retain their controls. Full JSON and inline parsing conflict.
- Validation: focused UI/CLI tests passed, including every split position in literal text, a JSON string and an unfinished tag; the four existing parser tests now explicitly opt in. The existing CLI hidden-inline test opts in too.
- Updated root/chat help, README descriptions/reference and Ollama examples, changelog and repository skill.

### B3 — Output errors

- Renderer writes retain errors, stop callbacks and propagate short writes, final newline and flush errors. CLI checks answer/JSON/raw/stats writes. Additional inspection found model-info and catalog-table writes could also lose failures; corrected those and help/version output.
- Validation: focused tests passed for content/reasoning errors, short writes, final-newline errors and flush errors, plus stats/info/table writers. Completion regressions and README/help consistency still pass. Real /dev/full CLI checks are part of final probes.
- Documentation describes nonzero status and possible partial artifacts; use staged publication.

### B4 — Token accounting

- Anthropic conversion sums uncached input, cache reads and cache creation once; the stream's final output update preserves the input categories. Stats show cache categories as included in input; raw JSON remains unchanged. OpenAI-compatible totals are not increased.
- Validation: two native Anthropic usage mocks (stream and buffered) report 130 input / 5 output / 135 total for 10 + 100 + 20 + 5; an OpenAI-compatible cached-input fixture verifies no double count. Focused client/UI/CLI tests passed.
- Integration observation: completion validation must inspect the rendered output when inline parsing/only-reasoning is active. Buffered text now renders into temporary buffers before validation/publication; stream metadata uses actual displayed content. Three regressions cover parsed reasoning-only success and hidden-reasoning-only failure, including a stream.

### B5 — Build toolchain

- CI tests the latest supported Go 1.26.x/1.27.x patches on amd64 with race checks; release/cross-build jobs use 1.27.2. Language minimum stays 1.22.
- Official [Go release history](https://go.dev/doc/devel/release) was rechecked. Downloaded the verified Go 1.27.2 toolchain into /tmp; `go version` reports linux/arm64. Installed host compiler remains unchanged.
- actionlint passed. The first 1.27.2 full-suite attempt failed: user-agent stream mocks returned only stop markers, and timeout mocks returned `{}`. These no longer satisfy B1. Original output retained in repair-go127-test.txt; fixtures were corrected and both patched compiler suites subsequently passed. This is not evidence of a Go compiler failure.

### B6 — Native Claude request compatibility

- Bounded profiles cover dated snapshots of Claude 4.6 and known 5.x/Opus 4.7-4.8 IDs. --effort sends adaptive thinking and output_config.effort; explicit caps remain intact. Manual thinking budgets remain supported on legacy models and 4.6; new profiles reject them. New profiles reject sampling flags; unknown/custom names keep the legacy profile.
- Validation: adaptive wire JSON (no budget_tokens), caps, six model variants, effort acceptance/rejection, legacy 6144/4096 cap/budget behavior and README/help synchronization passed in focused tests.
- Sources rechecked: [thinking modes/sampling](https://platform.claude.com/docs/en/build-with-claude/thinking), [model effort availability](https://platform.claude.com/docs/en/build-with-claude/effort). No live API rejection/success measured. Returning summarized thinking on models that omit it requires a future display option or a raw request; local --reasoning remains a rendering control.

### B7 — Protocol-aware export guard

- Native Anthropic Zed/Kilo/Continue exports are rejected before fetching a catalog. Protocol metadata is independent of the user-renamed section ID; the export library also guards non-OpenAI protocols. Table and raw JSON remain supported; unknown capability/output fields survive JSON export.
- Validation: three editor guards, raw native model capabilities/max-output preservation, and retained OpenAI rendering passed. Final probes will verify explicit native proxies and all three CLI export errors. Editor imports remain untested; implementing native adapter syntax is a follow-up.

### B8 — OpenAI-compatible model options

- Bounded GPT-6 profiles and snapshots normalize token caps and validate efforts/sampling; Sol/Luna support none, Astra/6.1 Sol do not. Omitted effort does not invent a new default. Unknown/custom names retain the legacy explicit behavior. o-series reject top-p as well as temperature. Recognized Claude gateway efforts use their model's accepted levels.
- Validation: 15 model/effort/sampling scenarios plus OpenRouter none translation and retained token cap passed, as did earlier request normalization, repaired user-agent/timeout fixtures and README/help synchronization.
- Sources: [GPT-6 compatibility](https://developers.openai.com/api/docs/guides/latest-model), [6.1 Sol capabilities](https://developers.openai.com/api/docs/models/gpt-6.1-sol), [Chat Completion token parameters](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create). Current docs and captured requests establish compatibility intent, not live model success. Responses/tools and default promotion remain separate follow-ups.

### B9 — Provider selection diagnostics

- All API command missing-key messages name the selected preset and list other configured variable names/flags. No credential values are printed and no provider is switched. Persistent defaults/config-explain remain a feature proposal.
- Validation: redaction, OpenAI-only/Qwen-alias guidance and unchanged fallback passed; documented priority tests and README/help synchronization also passed. Final isolated CLI probe uses only OPENAI_API_KEY.

### O1 — Examples and live-test observations

- Straitly and OpenRouter examples now explicitly select their providers; make info selects the matching OpenRouter catalog. Existing inline-reasoning audit scenarios explicitly opt into parsing, preserving the earlier stored baseline artifacts.
- The paid live script uses strict results and checks provider JSON/usage/reasoning metadata with jq. It accepts omitted native thinking blocks, readable compatible reasoning fields or positive reasoning-token metadata. A stderr notice cannot satisfy the assertion. It still makes at most two generation calls per configured preset; no paid execution was performed.
- Validation: ShellCheck passed; six fake-CLI fixtures passed (three valid reasoning forms, notice-only refusals and zero usage). Added a runnable guarded JSON example; all ten fake-provider fixtures and ShellCheck passed, including atomic publication and preservation of an existing output after failure.

### Integration review observations

- Applying adaptive thinking to Claude 4.6 also requires validating its sampling restrictions; a focused regression now rejects non-default temperature with effort. New profiles already reject sampling flags.
- Nullable legacy function_call fields must not be mistaken for actual tools. Review caught this before delivery; a CLI regression covers null function_call/tool_calls on an ordinary successful answer.
- Public help lists the effort levels supported by implemented profiles; minimal is recognized as a parameter value but rejected by current profiles, and is not advertised as an available level.

## Implementation index

| Work | Implementation | Focused regressions |
|---|---|---|
| B1 | [completion metadata/checks](/var/www/straitly/internal/client/completion.go), [CLI output](/var/www/straitly/cmd/callm/main.go), [native stream conversion](/var/www/straitly/internal/client/anthropic.go) | [completion CLI cases](/var/www/straitly/cmd/callm/completion_test.go) |
| B2–B3 | [renderer](/var/www/straitly/internal/ui/format.go), [catalog rendering](/var/www/straitly/internal/ui/table.go) | [literal/parser cases](/var/www/straitly/internal/ui/format_test.go), [failing writers](/var/www/straitly/internal/ui/write_test.go) |
| B4 | [native usage conversion](/var/www/straitly/internal/client/anthropic.go) | [cache/no-double-count fixtures](/var/www/straitly/internal/client/usage_test.go) |
| B5 | [CI](/var/www/straitly/.github/workflows/ci.yml), [release](/var/www/straitly/.github/workflows/release.yml) | Patched compiler test outputs and cross-build results below |
| B6/B8 | [bounded profiles](/var/www/straitly/internal/client/capabilities.go), native/compatible request conversion | [model parameter cases](/var/www/straitly/internal/client/capabilities_test.go) |
| B7 | [protocol export guard](/var/www/straitly/internal/export/export.go) | [export tests](/var/www/straitly/internal/export/export_test.go) |
| B9 | CLI missing-key diagnostic | [redaction and selection tests](/var/www/straitly/cmd/callm/default_test.go) |
| O1 | [live script](/var/www/straitly/scripts/test_live.sh), [guarded JSON example](/var/www/straitly/examples/guarded-json.sh) | [live-assertion fixtures](/var/www/straitly/audit/2026-10-09/check_live_script.py), [example checker](/var/www/straitly/audit/2026-10-09/check_example.py) |

## Validation and limits

The original report probes assert the baseline and should be run against the original build only. The separate repair probes assert current behavior. The legacy audit now explicitly opts into inline parsing for its inline-reasoning cases; its earlier stored JSON evidence is unchanged.

- Full tests passed on official Go 1.27.2 and 1.26.9, both natively on Linux ARM64 with localhost mocks. [1.27.2 coverage output](repair-go127-final-test.txt), [1.26.9 output](repair-go1269-test.txt). Go 1.27.2 coverage: CLI 75.9%, client 79.8%, config 80.9%, export 86.3%, UI 82.5%.
- Vet passed with no diagnostics: [log](repair-vet.txt). Workflow and shell lint passed: [actionlint](repair-actionlint.txt), [ShellCheck](repair-shellcheck.txt). Empty logs represent observed successful commands, not unperformed checks.
- [91 existing scenarios](repair-legacy-probes.json) and [31 repair probes](repair-probes.json) passed their assertions. Probes exercise real nonzero exits for /dev/full in buffered/stream/JSON/raw/info/table output and capture dummy request shapes; captured success is not live model acceptance.
- [Ten guarded-output fixtures](repair-example-checks.json) and [six live-script assertion fixtures](repair-live-script-checks.json) passed. They execute fake CLIs, not provider generation.
- Both installed skills matched the repository HEAD before copying; [synchronization hashes/backups](repair-skill-sync.json) record exact updates. Older installed binaries need a new build for these unreleased options; the skills explicitly say to check binary help.
- Race checks remain configured for Linux amd64 CI and were not executed here because of the recorded ARM runtime/VMA limitation. No remote CI run, native macOS/Windows run, paid provider generation, editor import, vulnerability database scan or release publication is claimed.

[Five help surfaces](repair-help-checks.json) passed. [Five release targets](repair-crossbuild.json) compiled successfully from the final reviewed source: Linux amd64/arm64, macOS amd64/arm64, Windows amd64. These artifacts were not executed on those target systems; the separately built Linux ARM64 binary was executed by the repair probes. Next implementation recommendations are in [NEXT_ACTIONS.md](NEXT_ACTIONS.md).

### Completion record

All B1–B9 repair entries and O1 are complete within the stated scope. Documentation was consolidated into user-facing behavior/reference sections and concise release notes while this log retains the individual decisions and observations. Provider/model defaults were not promoted. No system binary was installed and no release was published; the working-tree build is required for the unreleased flags.

The final tests/probes, help synchronization, workflow/shell lint and diff-whitespace checks passed. [Validation inventory and source hashes](repair-validation.json) make the reviewed files and artifacts identifiable. Future feature/evaluation work is listed separately in [NEXT_ACTIONS.md](NEXT_ACTIONS.md).
