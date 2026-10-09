# Pipeline examples

`guarded-json.sh` requires a build with `--strict`, `jq`, and OpenRouter credentials.
It requests the current OpenRouter preset model, checks completion status and the
expected JSON field, and publishes the output only after all checks pass:

```bash
PATH="$(pwd)/bin:$PATH" bash examples/guarded-json.sh input.txt summary.json
```

Build the working tree first with `make build`. The destination directory must
already exist. Pass explicit model settings in the script for a reproducible
scheduled job; the preset may change in a later release. A provider response is
not an instruction to execute. These examples make paid calls when run against a
real provider; validation here used fake-provider fixtures.

`--strict` checks completion semantics. `--json-object` requests JSON syntax;
`jq` enforces this example's required field. Neither option claims full JSON
Schema validation. An existing destination survives provider or validation failure.
Native Anthropic uses another response envelope and needs another extraction filter.
