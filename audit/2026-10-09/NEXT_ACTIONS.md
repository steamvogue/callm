# Suggested next actions after the repairs

Date: 2026-10-09. Implementation and evidence: [repair progress](PROGRESS.md).
Original numbered recommendations follow, with delivery updates. Later user
instruction: no paid models in tests. See [pipeline progress](PIPELINE_PROGRESS.md)
and [free evaluation](FREE_EVALUATION.md) for completed follow-up work.

1. **Finish security and platform validation.** The configured Go 1.26/1.27 amd64
   race/test jobs and all five builds passed on release commit a7b0bd0; v0.10.0
   is published with verified archive checksums. [Release evidence](RELEASE.md).
   A pinned govulncheck scan remains a follow-up. Local native ARM tests and
   cross-compilation do not establish race safety, native macOS/Windows behavior,
   or a clean vulnerability scan. Refresh the release compiler pin when a newer
   supported security patch appears.
   Update: repair commit 469f25b passed remote amd64 race/test jobs and all five
   build jobs. Local race execution remains impossible; no vulnerability scan
   pass is claimed. Review supported GitHub action revisions and runner pinning
   after the release workflow's Node 20 and ubuntu-latest migration annotations.

2. **Evaluate free candidates individually.** Completed 24 free calls across four
   models and six tasks, without retries. Gemma passed 5/6; no production defaults
   were promoted. Expand the reproducible task set using only catalog-verified
   zero-price IDs or local models/mocks. Record requested/returned IDs, validators,
   failures, latency and usage; separate formatting and availability from task
   quality. Do not use paid models or fallback in any test. Direct paid-provider
   acceptance remains untested and must not be inferred from gateway fixtures.

3. **Finish practical pipeline input and validation.** Add raw body/system/prompt
   files, a combined context-size limit, JSON Schema with local validation, and a
   stable result envelope containing status/finish/refusal/model/usage/timing.
   Preserve the existing raw JSON output. The guarded example is usable now;
   `--strict` checks completion semantics, not task correctness or JSON Schema.
   Update: implemented file inputs, combined body-size limit, offline schema
   validation and versioned results. The two-step mini harness adds atomic
   artifacts, journal/hash resume and bounded calls. Extend it with deterministic
   domain validators; a model's self-check cannot establish factual correctness.

4. **Make configuration and diagnostics reproducible.** Add a persistent explicit
   provider/model profile and a redacted offline configuration explanation.
   Extend capability profiles from documented model metadata, with pinned
   configuration and custom-model overrides. Add a native Claude thinking-display
   option when the workflow needs summaries; local --reasoning does not request
   them from models whose provider default omits thinking. Validate native editor
   adapters in the actual editors before replacing the export guard.

5. **Add bounded retries, then the mini harness.** Classify errors and retry only
   explicitly eligible failures with attempt/deadline limits. Refusals and exposed
   partial streams are not automatic retry candidates. Start the harness with a
   versioned JSON manifest and sequential prepare/call/validate/save steps, an
   optional single repair, atomic artifacts, a journal and hash-based resume.
   Add batch concurrency and token visibility after that contract is stable.
   Model-controlled tools and Responses conversations require separate protocol
   work and retained reasoning/tool state.
   Update: delivered the sequential mini harness as an optional Python example
   with one attempt per step and explicit manual resume. Automatic retry/repair,
   concurrency and global token/deadline budgets remain follow-ups. Free-only
   test constraints apply to any future retry/evaluation tests.

Acceptance for the next stage should include a useful end-to-end pipeline example,
fixture-tested failure handling, documented behavior and synchronized skills.
Keep mock evidence, real provider request checks and quality benchmarks distinct.

Next priorities after this delivery: deterministic validators for actual pipeline
data; a reproducible free-only structured-output/provider-parameter capability
matrix (one live probe already rejected invalid output); persistent explicit
provider/model profiles with offline redacted diagnostics; and bounded retry/repair
policies with a global call/token budget. The schema library's measured startup
increase is documented—profile it before pursuing a smaller implementation.
