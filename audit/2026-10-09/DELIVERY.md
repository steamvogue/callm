# Delivery record

Date: 2026-10-09. All authorized work is committed/pushed to main. No release tag
or published release was created.

- `469f25b`: original B1–B9 repairs, examples and research/progress evidence.
- `cc4c776`: file inputs, combined size limits, JSON Schema/local validation,
  versioned result envelopes, optional mini harness and free-only live tests.
- `73cafd8`: shared CLI fixture race-runtime exit settings after remote CI exposed
  a cumulative timeout. Production behavior/source did not change in this fix.

The final code commit passed [remote CI](https://github.com/steamvogue/callm/actions/runs/37929256975):
both Go 1.26/1.27 amd64 race/test jobs, their offline pipeline/free-guard fixtures,
and five target builds. [Run metadata](final-code-ci.json), [jobs](final-code-ci-jobs.json).
The initial failed pipeline CI log remains in [the progress record](PIPELINE_PROGRESS.md).
Local race execution was not attempted on the unsupported ARM/VMA host.

Built and installed `v0.9.0-3-g73cafd8` natively with official Go 1.27.2 into
/usr/local/bin/callm and /home/lordtime/.local/bin/callm. Both straitly aliases
resolve to the matching callm copies. Build and both installed binaries share
SHA-256 `0151bf982b2f582d4b950993b4d14e3889fbdab585a70ffe4154d38990b24ebf`.
Verified versions/help and both installed skill copies. [Installation evidence](installation-final.json)
includes the exact paths, binary/skill hashes and CI commit. Earlier installation
evidence is preserved in [installation.json](installation.json). Seven harness
fixtures also passed on the installed feature binary before the test-only fix.

All 27 live request attempts used explicit catalog-verified free models: 24
evaluation, two successful smoke calls and one schema attempt rejected locally.
No paid model calls, judge calls, retries or paid fallback occurred. The rejected
schema probe is retained as a failure; the later routing guard is mock-tested.
Production defaults remain unchanged. [Evaluation](FREE_EVALUATION.md),
[implementation/observations](PIPELINE_PROGRESS.md), [next priorities](NEXT_ACTIONS.md).

This delivery-record commit changes audit documentation only; the installed
binary identifies the verified code commit above. No clean vulnerability scan,
native execution on the other build targets, native paid-provider compatibility
or editor import is claimed.
