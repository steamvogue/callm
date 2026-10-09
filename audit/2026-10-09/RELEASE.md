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

Pending at this preparation checkpoint: commit/main push, annotated tag push,
and verification of the resulting GitHub release build and downloadable assets.
Native execution on other platforms is not inferred from cross-compilation.
