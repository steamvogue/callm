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
not an instruction to execute. The guarded script uses the ordinary OpenRouter
default and can incur charges; it was tested only with fake-provider fixtures.

`--strict` checks completion semantics. `--json-object` requests JSON syntax;
`jq` enforces this example's required field. Neither option claims full JSON
Schema validation. An existing destination survives provider or validation failure.
Native Anthropic uses another response envelope and needs another extraction filter.

## File/schema inputs and stable results

With an explicitly chosen schema-capable model:

```bash
callm --or -m google/gemma-4-26b-a4b-it:free --no-stdin \
  --prompt-file examples/pipeline/extract.txt --system-file examples/pipeline/system.txt \
  -f examples/pipeline/invoice.txt --validate-schema examples/pipeline/invoice.schema.json \
  --result-json --max-tokens 2048 > result.json
jq -e '.status == "ok" and .schema_valid == true' result.json
```

Check the callm exit code before the jq check; use shell `set -e` or an explicit
conditional. `--validate-schema` rejects malformed/nonconforming answers locally.
Use `--schema` instead to request provider constraints too; provider subsets vary.
Neither schema validation nor a model's self-check proves factual correctness.
Do not run examples against a paid model as a test.

## Mini harness (Python 3, Linux/macOS)

```bash
python3 examples/mini-harness.py examples/pipeline/manifest.json \
  --callm ./bin/callm --output-dir /tmp/callm-invoice-run
python3 examples/mini-harness.py examples/pipeline/manifest.json \
  --callm ./bin/callm --output-dir /tmp/callm-invoice-run --resume
```

The supplied manifest selects an explicit `:free` model. Confirm its current
zero prices/access before running it; `make test-live` does this for its smoke
tests. The general harness accepts an operator-selected model and does not enforce
free pricing. Use localhost mocks in tests (`scripts/test_mini_harness.py`).

Manifest version 1 requires provider, explicit API URL/model and 1–16 sequential
steps. Optional `key_env` names a credential variable; never embed a key value.
`max_tokens` defaults to 2048 (1–16384), `timeout_seconds` to 60 (1–600). Each step
requires a simple unique `id`, `prompt_file` and `schema`; optional `system_file`
and up to 32 `inputs` are supported. Paths are relative to the manifest directory.
`step:ID` references an earlier step's JSON output. Unknown fields, duplicate IDs,
forward references and unsupported versions fail. Schemas are local validation
only; required structure is also described in the prompts.

There is at most one generation per step per invocation, no automatic repair or
retry, and failure stops later steps. Failed results are saved for inspection;
the previous answer artifact survives and its journal entry becomes an error.
Each successful step writes `ID.json` and `ID.result.json`; `journal.json` records
input/recipe/output SHA-256 hashes. Writes use temporary files in the destination
directory, flush/sync and rename; journal completion is written last. An interrupted
step can be called again on resume—this is not an exactly-once provider guarantee.

Resume verifies manifest/settings, runner/binary hashes, input bytes and both
artifacts; changed/tampered steps run again. The runner sends snapshots of the
exact fingerprinted inputs. One lock prevents concurrent writers; after forced
termination, remove `.lock` only after checking that no runner remains. Output
files may contain sensitive prompts/answers; keep the output directory private.
No model response is executed as a command. This example has no parallelism,
global token budget, provider fallback or Responses/tool conversation loop.
