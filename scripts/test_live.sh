#!/usr/bin/env bash
set -e

CALLM="./bin/callm"
if [ ! -f "$CALLM" ]; then
  CALLM="callm"
fi

printf "\033[1;36m=== Running Live API Call Tests (Minimal + Reasoning) ===\033[0m\n\n"

# 1. Straitly
if [ -n "$STRAITLY_API_KEY" ] || [ -n "$CALLM_API_KEY" ]; then
  printf "\033[1m[Straitly Gateway]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --st --no-reasoning "Reply with: OK_STRAITLY_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call: "
  "$CALLM" --st --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[Straitly Gateway] SKIPPED (no STRAITLY_API_KEY)\033[0m\n"
fi

# 2. OpenRouter
if [ -n "$OPENROUTER_API_KEY" ]; then
  printf "\033[1m[OpenRouter Gateway]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --or -m deepseek/deepseek-chat --no-reasoning "Reply with: OK_OPENROUTER_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call (DeepSeek-R1): "
  "$CALLM" --or -m deepseek/deepseek-r1 --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[OpenRouter Gateway] SKIPPED (no OPENROUTER_API_KEY)\033[0m\n"
fi

# 3. DeepSeek Direct
if [ -n "$DEEPSEEK_API_KEY" ]; then
  printf "\033[1m[DeepSeek Direct API]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --ds --no-reasoning "Reply with: OK_DEEPSEEK_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call (deepseek-reasoner): "
  "$CALLM" --ds -m deepseek-reasoner --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[DeepSeek Direct API] SKIPPED (no DEEPSEEK_API_KEY)\033[0m\n"
fi

# 4. OpenAI Direct
if [ -n "$OPENAI_API_KEY" ]; then
  printf "\033[1m[OpenAI Direct API]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --oa -m gpt-4o-mini --no-reasoning "Reply with: OK_OPENAI_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call (o3-mini): "
  "$CALLM" --oa -m o3-mini --effort=low "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[OpenAI Direct API] SKIPPED (no OPENAI_API_KEY in environment)\033[0m\n"
fi

# 5. Moonshot AI (Kimi)
if [ -n "$MOONSHOT_API_KEY" ]; then
  printf "\033[1m[Moonshot AI]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --ms --no-reasoning "Reply with: OK_MOONSHOT_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call: "
  "$CALLM" --ms --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[Moonshot AI] SKIPPED (no MOONSHOT_API_KEY in environment)\033[0m\n"
fi

# 6. Zhipu AI (GLM / ZAI)
if [ -n "$ZAI_API_KEY" ] || [ -n "$ZHIPU_API_KEY" ]; then
  printf "\033[1m[Zhipu AI (GLM)]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --zai --no-reasoning "Reply with: OK_ZHIPU_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call: "
  "$CALLM" --zai --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[Zhipu AI (GLM)] SKIPPED (no ZAI_API_KEY in environment)\033[0m\n"
fi

# 7. Alibaba Cloud (Qwen)
if [ -n "$DASHSCOPE_API_KEY" ] || [ -n "$QWEN_API_KEY" ]; then
  printf "\033[1m[Alibaba DashScope (Qwen)]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --qw --no-reasoning "Reply with: OK_QWEN_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call (qwq-32b): "
  "$CALLM" --qw -m qwq-32b --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[Alibaba DashScope (Qwen)] SKIPPED (no DASHSCOPE_API_KEY in environment)\033[0m\n"
fi

# 8. Groq OSS
if [ -n "$GROQ_API_KEY" ]; then
  printf "\033[1m[Groq Ultra-Fast OSS]\033[0m\n"
  printf "  --> Minimal call: "
  out=$("$CALLM" --groq -m llama-3.3-70b-versatile --no-reasoning "Reply with: OK_GROQ_MINIMAL")
  printf "\033[32m%s\033[0m\n" "$out"

  printf "  --> Reasoning call (DeepSeek-R1-distill): "
  "$CALLM" --groq -m deepseek-r1-distill-llama-70b --reasoning "Which is larger: 9.11 or 9.9? Reply in one short sentence."
  printf "\n"
else
  printf "\033[33m[Groq Ultra-Fast OSS] SKIPPED (no GROQ_API_KEY in environment)\033[0m\n"
fi

# 9. Anthropic Direct (Excluded per instructions)
printf "\033[90m[Anthropic Direct API] EXCLUDED (Anthropic not available in system)\033[0m\n"

printf "\n\033[1;32m=== All Live API Tests Completed Successfully ===\033[0m\n"
