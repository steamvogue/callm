# Installation, free evaluation and pipeline progress

Continues [the completed repairs](PROGRESS.md) and [original report](REPORT.md).
User authorized commit/push, local compilation/installation, then recommendation
2 (evaluation) and 3 (pipeline features), with the later explicit constraint:
**no paid models in any test**. The mini harness is an optional example building
on recommendation 3; automated retry/repair/concurrency is deferred.

## Repair delivery

- Committed/pushed 469f25b to main after fetching origin; branches were identical
  before the commit. Checked 61 changed/new files against exported credential
  values without printing them; no matches.
- Compiled natively with official Go 1.27.2 and installed the same binary into
  /usr/local/bin/callm and /home/lordtime/.local/bin/callm (which shadows it on PATH).
  All three build/installation hashes were
  9b1a11819b29c4589bad429a4c81d4b50500113257c7998fa1f069a83e7410cc.
  Version reported v0.9.0-1-g469f25b. Backups are in
  /tmp/callm-install-backups-20261009; temporary host cleanup can remove them.
- Direct installation initially failed on system directory ownership; scoped
  noninteractive sudo succeeded. The user copy was also updated. No release tag
  or published release was created.
- Did not retry local race execution: this ARM host has the documented runtime
  limitation. [Repair CI](https://github.com/steamvogue/callm/actions/runs/37924720634)
  completed successfully, including both amd64 Go race jobs and five cross-build
  jobs. [Saved job metadata](repair-remote-ci.jsonl) is remote evidence, not local
  race execution or native execution of all cross-built targets.

## Evaluation

- No paid generation had started when the user clarified the constraint.
- Completed the bounded [24-call free evaluation](FREE_EVALUATION.md).
  Production defaults remain unchanged; the free example/smoke candidate is
  explicit. Preserve returned-model, latency, exact validator failures and raw
  usage rather than treating API success as task correctness.
- Changed make test-live from all-provider paid generation to two explicit free
  OpenRouter calls with fresh catalog verification. Missing/malformed/nonzero
  pricing, unavailable key and non-free IDs fail/skip before generation. No paid
  model, retry or provider fallback. Historical repair live-script fixtures/logs
  describe commit 469f25b; new fixture script is scripts/test_free_live.py.

## Pipeline implementation, one step at a time

1. Added prompt/system regular-file flags and raw body files/stdin. System flag
   conflicts are explicit; prompt whitespace is preserved. Raw rejects invalid
   JSON, extra bodies and conflicting argument/file sources.
2. Added a combined input and actual serialized wire-body cap, default/maximum
   64 MiB, lowerable with --max-input-bytes. Counts file wrappers, stdin, system
   text, images/encoding and JSON/schema bytes. This is not a token estimate.
3. Added pinned jsonschema/v6 v6.0.3 local validation instead of an improvised
   keyword subset. Default Draft 2020-12, built-in format assertions, supported
   declared older drafts, 1 MiB schema cap and an offline resource loader. Internal
   references work; external file/HTTP resources do not load. Added upstream
   license/patent texts and release archive notices/examples. Single-binary runtime
   remains; build now downloads Go dependencies.
4. --schema sends the original strict compatible/native schema and checks answers
   locally. --validate-schema omits provider schema constraints for models that
   cannot accept them. Neither rewrites unsupported provider keywords. Both imply
   buffered strict completion validation; malformed/mismatched output is withheld.
5. --result-json emits a versioned strict result envelope. Errors can contain
   partial answers; consumers must require successful exit and status ok. Null
   usage/schema result means absent/not checked, not a fabricated zero/success.
   Retained provider cache/reasoning details in normalized usage; original --json
   remains available. Preflight/input/credential failures may emit no envelope.
6. Added the Python sequential mini harness and two-step invoice example. It pins
   API/model, validates every call locally, snapshots fingerprinted input bytes,
   writes artifacts and journal atomically, locks concurrent writers and checks
   input/recipe/output hashes on resume. Maximum 16 calls per invocation; one
   attempt per step. No automatic repair/retry or model-controlled command execution.

## Observations during implementation

- Initial focused tests were blocked by sandbox localhost-socket restrictions;
  reran with scoped approval and mocks. This is distinct from the ARM race issue.
- Module tidy needed a network-enabled fetch for the validator's test-only
  regexp2 dependency. It is not linked into the CLI.
- Review caught a catalog shape mismatch: callm's catalog export is a JSON array,
  not the provider's data wrapper. Corrected the free guard and fixture shape
  before any live smoke test.
- Review also preserved prompt-file whitespace and required an integer manifest
  version. Resume snapshots avoid attributing changed-on-disk input to an old hash.
- A changed upstream prompt can be recomputed without rerunning a later step when
  its actual structured input is unchanged. A changed manifest/runner/binary
  invalidates the recipe. Interrupted remote calls can repeat on manual resume;
  there is no exactly-once provider guarantee.
- This host's gh lacks run-list --commit. Used its supported REST API interface
  and saved the successful remote CI job metadata. Tool reuse notes updated.
- Two free-only live smoke calls passed. One separate free --schema call produced
  a complete but invalid fenced answer with invoice_id instead of invoice; local
  validation emitted status error/schema_valid false and nonzero exit. Reported
  usage was 29 input + 39 output = 68 tokens, cost 0. [Raw evidence](free-schema-live.json).
  This is an observed validation rejection, not a successful structured-output
  capability check. It suggests routing/parameter support rather than proving
  which backend behavior caused it. Added OpenRouter provider.require_parameters
  for schema requests, following the official routing contract; kept local checks.
- [Startup benchmark](pipeline-startup-benchmark.json): 3 warmups/25 runs each,
  shell-free --version, repair build 2.618 ms mean vs pipeline build 8.275 ms mean.
  This includes process startup and schema-library initialization on this host,
  not network generation. The absolute ~5.7 ms increase is accepted for a complete
  embedded schema validator; it is a measurable regression, not a performance win.

## Validation and remaining limits

Final source passed native Go 1.27.2 tests/coverage and vet, native Go 1.26.9 tests,
help synchronization, workflow lint and shell lint. [Go 1.27.2](pipeline-go-test.txt),
[Go 1.26.9](pipeline-go1269-test.txt), [vet](pipeline-vet.txt),
[actionlint](pipeline-actionlint.txt), [ShellCheck](pipeline-shellcheck.txt).
Empty lint logs correspond to observed successful commands. Pipeline coverage is
86.7%; CLI coverage 78.4%.

[Six free-guard fixtures](free-live-mock-checks.json) passed without provider calls;
non-free/missing/malformed/nonzero prices caused zero fake generation calls.
[Seven mini-harness fixtures](harness-mock-checks.json) passed against the compiled
CLI and localhost server, including resume, input/tamper detection, failure
preservation and forward-reference rejection. CI runs these fixtures without
provider credentials. Buffered error status, schema/refusal/truncation failures,
raw files/stdin deadlines, native schema/effort coexistence, encoded wire limits
and parameter-aware OpenRouter requests are covered by Go regressions.

[Two free live smoke calls](free-live-smoke.txt) passed exact answer, terminal
reason, positive output-usage and zero-cost checks. The separate free schema
attempt failed its quality/capability check and passed rejection behavior, as
recorded above. There were 27 free-only attempts total in this stage (24 evaluation,
2 smoke, 1 schema), no paid calls and no retries. The final routing guard was
mock-tested; no additional live attempt was made after adding it.

Both installed agent skills were synchronized with verified baseline copies,
backups and [hashes](pipeline-skill-sync.json). [Validation/source inventory](pipeline-validation.json)
identifies the implementation snapshot and evidence. The historical live checker
now retrieves its original 469f25b script; it still passes six fake fixtures and
does not accidentally test the changed current script. No native paid
OpenAI/Anthropic request, editor import, paid benchmark, local race pass or clean
vulnerability scan is claimed. Schemas establish structure, not factual correctness.
The optional harness needs Python 3 on Linux/macOS; the CLI remains portable Go.
Its failure journal/lock are documented, and no global token budget or automatic
retry/repair is implemented. Final installation and source hashes are recorded
separately after committing the implementation.

Sources: [validator API/features](https://github.com/santhosh-tekuri/jsonschema),
[Anthropic structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs),
[OpenRouter structured outputs](https://openrouter.ai/docs/guides/features/structured-outputs).
[OpenRouter parameter routing](https://openrouter.ai/docs/guides/routing/provider-selection)
documents the new schema routing guard.
Provider documentation guided wire conversion; local mocks establish request
shape, not live native-provider acceptance.
