#!/usr/bin/env bash
# Requires callm v0.10.0+ with --strict, jq, and OPENROUTER_API_KEY.
set -euo pipefail
input=${1:-input.txt}
destination=${2:-summary.json}
reply=$(mktemp)
staged=''
trap 'rm -f -- "$reply" "$staged"' EXIT
staged=$(mktemp "${destination}.tmp.XXXXXX")

callm --or --strict --json --json-object --no-stdin \
  --timeout 90s --max-tokens 4096 -f "$input" \
  'Return a JSON object with a nonempty string field named summary.' >"$reply"

jq -e '
  if .choices[0].finish_reason != "stop" then error("incomplete response") else . end
  | .choices[0].message
  | if (.refusal // "") != "" then error("refusal") else .content end
  | fromjson
  | if type != "object" then error("expected object") else . end
  | if (.summary | type) != "string" then error("expected summary") else . end
  | if (.summary | length) == 0 then error("empty summary") else . end
' "$reply" >"$staged"

mv -- "$staged" "$destination"
