# v0.10.0 release record

Date: 2026-10-09. The owner requested a release tag so GitHub CI compiles and
publishes the new version. v0.9.0 is the latest published version; v0.10.0 is
selected for the added pipeline options, JSON Schema validation, result envelopes,
optional mini harness and compatibility fixes. Production model defaults remain
unchanged. All provider tests must use free models; no live calls are needed here.

## Preparation

- Refreshed origin/main and all remote tags; local main matches origin/main, and
  v0.10.0 does not already exist.
- The starting commit `e4268cc` passed [GitHub CI](https://github.com/steamvogue/callm/actions/runs/37930190936).
- Promoted the changelog to v0.10.0 and updated version references in README,
  the guarded example and the distributed agent skill. Historical audit records
  retain their original evidence.
- The existing `v*` tag workflow compiles Linux amd64/arm64, macOS amd64/arm64
  and Windows amd64 with Go 1.27.2, then publishes archives and SHA-256 checksums.
  Each archive includes documentation, examples, the agent skill and licenses.

## Validation and publication

- Passed `go test ./...` and `go vet ./...` natively with Go 1.27.2. Tests use
  mocks; no live provider requests were made.
- Passed six fake-only free-live guard fixtures and seven localhost harness
  fixtures against the existing native build of `73cafd8` (production code is
  unchanged by this release documentation update).
- Passed actionlint, ShellCheck for the updated guarded example and whitespace
  checks. No unreleased labels remain in distributed documentation/examples.
- Synchronized both installed agent skill copies, with backups in
  `/tmp/callm-v010-skill-backups`. Repository, Codex and Claude skill SHA-256:
  `37db70541e656704472d28a7b6ebc4af1e9b6b747567931bf9287b56f5e071a8`.

## Published outcome

- Committed/pushed preparation as `a7b0bd04591049b4a4ef82ac358cb4aa539cec82`.
  Pushed annotated tag v0.10.0; remote tag object
  `e330f7daf9978faff4247bedf5bd9c5257fe92ae` resolves to that commit. The tag remains
  on the release commit; this subsequent evidence update does not move it.
- [Release build](https://github.com/steamvogue/callm/actions/runs/37931091350)
  completed successfully and published [v0.10.0](https://github.com/steamvogue/callm/releases/tag/v0.10.0)
  at 2026-10-09T12:37:08Z. Five platform archives and checksums.txt are public.
  [Run evidence](release-v0.10.0-ci.json), [job](release-v0.10.0-ci-jobs.json),
  [release/assets metadata](release-v0.10.0.json).
- [Release-commit test CI](https://github.com/steamvogue/callm/actions/runs/37931066707)
  passed both Go 1.26/1.27 amd64 race/test jobs (including offline fixtures) and
  all five cross-build jobs. [Run](release-v0.10.0-test-ci.json),
  [jobs](release-v0.10.0-test-ci-jobs.json).
- Downloaded all five published archives and verified every SHA-256 against the
  published checksums, plus asset sizes/API digests. Inspected each archive for
  its binary and byte-identical README, changelog, licenses, harness, manifest and
  agent skill. Executed only the downloaded Linux ARM64 binary: version v0.10.0,
  commit a7b0bd0, build time 2026-10-09T12:36:10Z; help contains the new options.
  [Asset validation](release-v0.10.0-assets-validation.json). Native execution of
  the other four targets is not claimed.
- Built v0.10.0 independently on this host with Go 1.27.2 and installed it in both
  existing system/user paths; both straitly aliases resolve correctly. Local build
  and installed binary SHA-256:
  `db4220f7f7680ce7d352c430797cef5ac87433c4b638a57f4a45fbc1d6c795ab`.
  Verified matching help, all subcommand help and seven localhost harness fixtures
  on the installed v0.10.0 binary. This local build has its own build timestamp;
  it is not claimed byte-identical to the GitHub build.
  [Installation evidence](installation-v0.10.0.json). Previous binary copies are
  retained in `/tmp/callm-v010-install-backups`.
- The workflow initially generated only a version compare link. Published the
  reviewed v0.10.0 changelog and download/runtime guidance as release notes;
  verified the text matches and all six assets remain present.
- No live provider requests or paid-model tests were made during release work.

## Observations and next actions

GitHub reported two non-failing [annotations](release-v0.10.0-annotations.json):
the checkout/setup-go/release action revisions target deprecated Node 20 and were
forced to Node 24, and ubuntu-latest is scheduled to migrate to Ubuntu 26 from
October 19. The build log also contains Node punycode/url.parse deprecation notices
from setup-go. Review supported action revisions and runner pinning in a separate
maintenance change; validate that change through the existing mocked CI checks.
No clean vulnerability scan or native macOS/Windows acceptance is claimed.
Product follow-ups remain in [next actions](NEXT_ACTIONS.md).
