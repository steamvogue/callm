#!/usr/bin/env bash
# Free-only smoke tests. Never fall back to a preset or a paid model.
set -euo pipefail
CALLM=${CALLM_TEST_BIN:-./bin/callm}
if [[ ! -x "$CALLM" ]]; then CALLM=callm; fi
if ! command -v jq >/dev/null 2>&1; then
  printf 'Free live validation requires jq.\n' >&2
  exit 2
fi
if [[ -z ${OPENROUTER_API_KEY:-} ]]; then
  printf 'Free live tests skipped: OPENROUTER_API_KEY unavailable.\n' >&2
  exit 2
fi
model=${CALLM_TEST_FREE_MODEL:-google/gemma-4-26b-a4b-it:free}
if [[ $model != *:free ]]; then
  printf 'Refusing a model without an explicit :free suffix.\n' >&2
  exit 1
fi
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
options=(--or --api https://openrouter.ai/api/v1 --api-key-env OPENROUTER_API_KEY --timeout 60s)
"$CALLM" "${options[@]}" models --format json >"$scratch/catalog.json"
# Require all reported price categories to be zero, and explicit prompt/output
# prices. Missing, malformed or nonzero pricing fails before any generation.
jq -e --arg model "$model" '
  [ .[] | select(.id == $model) ]
  | length == 1 and (.[0].pricing
    | (.prompt | tonumber) == 0 and (.completion | tonumber) == 0
      and (to_entries | length > 0 and all(.[]; (.value | tonumber) == 0)))
' "$scratch/catalog.json" >/dev/null || {
  printf 'Refusing model not confirmed zero-price by the current catalog.\n' >&2
  exit 1
}
for marker in OK_FREE_MINIMAL OK_FREE_PIPELINE; do
  "$CALLM" "${options[@]}" --model "$model" --no-stdin --strict --json \
    --max-tokens 2048 "Reply exactly: $marker" >"$scratch/reply.json"
  jq -e --arg marker "$marker" '
    .choices[0].finish_reason == "stop"
    and (.choices[0].message.content == $marker)
    and (.usage.completion_tokens > 0)
    and (.usage.cost == 0)
  ' "$scratch/reply.json" >/dev/null || {
    printf 'Free answer/usage/zero-cost assertion failed; no retry.\n' >&2
    exit 1
  }
done
printf '2 free live assertions passed (%s); no retries or paid fallback.\n' "$model"
