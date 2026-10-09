# Third-party notices

The compiled callm binary includes these pinned Go modules:

- `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3, Apache License 2.0.
  See [license](licenses/jsonschema-LICENSE).
- `golang.org/x/text` v0.14.0, Copyright The Go Authors, BSD license and patent
  grant. See [license](licenses/x-text-LICENSE) and [patents](licenses/x-text-PATENTS).

Exact upstream license texts are retained. The jsonschema package is used without
source modifications; callm supplies an offline resource loader and chooses
validation settings through its public API. Dependency versions/checksums are
recorded in go.mod/go.sum. The test-only regexp2 dependency is not linked into
callm. callm's own code remains licensed under the root MIT LICENSE.
