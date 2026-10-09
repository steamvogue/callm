#!/usr/bin/env bash
set -euo pipefail

CALLM=${CALLM_TEST_BIN:-./bin/callm}
if [[ ! -x "$CALLM" ]]; then CALLM=callm; fi
if ! command -v jq >/dev/null 2>&1; then
  printf 'Live response validation requires jq.\n' >&2
  exit 2
fi
passed=0
skipped=0
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

# Export provider credentials before running. Each call explicitly selects its key;
# CALLM_API_KEY cannot accidentally redirect another provider's authentication.
check_provider() {
  local preset=$1 key_env=$2 reasoning_model=${3:-} marker out
  if [[ -z ${!key_env:-} ]]; then
    printf '%s: skipped (%s unavailable)\n' "$preset" "$key_env"
    skipped=$((skipped + 1))
    return
  fi
  marker="OK_${preset}_MINIMAL"
  local minimal_tokens=64
  # deepseek-flash thinks by default; leave room for reasoning within max_tokens.
  if [[ $preset == ds ]]; then minimal_tokens=1024; fi
  out=$("$CALLM" "--$preset" --api-key-env "$key_env" --no-stdin --strict --no-stream --no-reasoning --max-tokens "$minimal_tokens" "Reply exactly: $marker")
  if [[ $out != *"$marker"* ]]; then
    printf '%s: minimal response did not contain the expected marker\n' "$preset" >&2
    return 1
  fi
  passed=$((passed + 1))
  if [[ -n $reasoning_model ]]; then
    local options=(-m "$reasoning_model")
    if [[ $preset == ant ]]; then options+=(--effort high); fi
    # Inspect actual provider JSON/usage. A dotenv notice or stats on stderr is
    # not proof that reasoning ran. Omitted thinking blocks still establish it.
    "$CALLM" "--$preset" --api-key-env "$key_env" --no-stdin --strict --json \
      "${options[@]}" --max-tokens 4096 \
      'Which is larger, 9.11 or 9.9? Finish with: 9.9 is larger.' >"$scratch/reply.json"
    jq -e '
      ((.choices[0].message.content // ([.content[]? | select(.type == "text") | .text] | join("")))
        | contains("9.9 is larger"))
      and ((.usage.completion_tokens // .usage.output_tokens // 0) > 0)
      and (
        any(.content[]?; .type == "thinking" or .type == "redacted_thinking")
        or ((.choices[0].message.reasoning // .choices[0].message.reasoning_content // .choices[0].message.thought // "") | length > 0)
        or ((.usage.completion_tokens_details.reasoning_tokens // 0) > 0)
      )
    ' "$scratch/reply.json" >/dev/null || {
      printf '%s: answer/usage/reasoning metadata assertion failed\n' "$preset" >&2
      return 1
    }
    passed=$((passed + 1))
  fi
  printf '%s: passed\n' "$preset"
}

st_key=STRAITLY_API_KEY
if [[ -z ${STRAITLY_API_KEY:-} && -n ${CALLM_API_KEY:-} ]]; then st_key=CALLM_API_KEY; fi
check_provider st "$st_key"
check_provider or OPENROUTER_API_KEY deepseek/deepseek-r1
check_provider orca ORCA_API_KEY
check_provider ds DEEPSEEK_API_KEY deepseek-flash
check_provider oa OPENAI_API_KEY
check_provider ms MOONSHOT_API_KEY
check_provider kimi KIMI_API_KEY
zai_key=ZAI_API_KEY
if [[ -z ${ZAI_API_KEY:-} ]]; then zai_key=ZHIPU_API_KEY; fi
check_provider zai "$zai_key"
qw_key=DASHSCOPE_API_KEY
if [[ -z ${DASHSCOPE_API_KEY:-} ]]; then qw_key=QWEN_API_KEY; fi
check_provider qw "$qw_key"
check_provider groq GROQ_API_KEY
check_provider pool POOLSIDE_API_KEY
check_provider ant ANTHROPIC_API_KEY claude-sonnet-4-6
if [[ $passed -eq 0 ]]; then
  printf 'No live tests ran; %d providers skipped.\n' "$skipped" >&2
  exit 2
fi
printf '%d live assertions passed; %d providers skipped.\n' "$passed" "$skipped"
