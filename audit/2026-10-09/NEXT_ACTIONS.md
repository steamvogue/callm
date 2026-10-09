# Suggested next actions after the repairs

Date: 2026-10-09. Implementation and evidence: [repair progress](PROGRESS.md).
These are recommendations, not completed work.

1. **Complete release validation on Linux amd64.** Run the configured Go 1.26/1.27
   race jobs and a pinned govulncheck scan. Review the result-status and inline-tag
   compatibility changes before publishing a release. Local native ARM tests and
   cross-compilation do not establish race safety, native macOS/Windows behavior,
   or a clean vulnerability scan. Refresh the release compiler pin when a newer
   supported security patch appears. No release was published here.

2. **Evaluate default candidates individually.** Start with a small representative
   batch comparing the old default and one candidate. Confirm direct-provider and
   gateway request acceptance separately. Then use the report's 24-case quality
   set with explicit model IDs/settings, validator results, failures, latency and
   returned token/cache accounting. The full proposal is at most 144 generation
   calls per changed provider, plus any separately counted judge calls. This is
   paid work and needs an explicit provider/call budget; no quality or cost win was
   measured in this repair pass. Priority candidates remain OpenAI 6.1 Sol/Luna,
   native Claude Sonnet/Haiku 5.5, OpenRouter DeepSeek V4.1 Flash, and accessible
   Groq GPT-OSS models. Keep regional endpoint/key pairing explicit.

3. **Finish practical pipeline input and validation.** Add raw body/system/prompt
   files, a combined context-size limit, JSON Schema with local validation, and a
   stable result envelope containing status/finish/refusal/model/usage/timing.
   Preserve the existing raw JSON output. The guarded example is usable now;
   `--strict` checks completion semantics, not task correctness or JSON Schema.

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

Acceptance for the next stage should include a useful end-to-end pipeline example,
fixture-tested failure handling, documented behavior and synchronized skills.
Keep mock evidence, real provider request checks and quality benchmarks distinct.
