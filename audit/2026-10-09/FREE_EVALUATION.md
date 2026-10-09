# Free-model evaluation

Date: 2026-10-09. No paid model was called. User instruction superseded the
earlier paid-comparison proposal before generation began.

## Method and limits

Twenty-four requests: four explicit OpenRouter `:free` IDs × six synthetic tasks,
one sample per case, no retries or judge calls. All catalog price categories were
verified zero before the run. Each call used the repair build at commit 469f25b,
an explicit OpenRouter endpoint/key variable, a 60-second timeout, 2048 output
tokens maximum and strict completion checks. No production source or credentials
were included in prompts. The six tasks cover invoice extraction, stable sorting,
configuration precedence, pipeline finish rules, literal tags and untrusted-data
instruction handling. JSON was requested in the prompt, without provider schema
constraints; exact parsed JSON equality is the validator. Markdown fences fail.

This is a small formatting/logic smoke batch, not the report's full 24-task
quality benchmark, a statistical comparison or evidence about paid defaults,
direct-provider acceptance, large code tasks or current availability elsewhere.

## Observed results

| Explicit free model | Exact task passes | Complete API replies | Median seconds for complete replies |
|---|---:|---:|---:|
| google/gemma-4-26b-a4b-it:free | 5/6 | 6/6 | 1.307 |
| apodex/apodex-1.1-mini:free | 4/6 | 5/6 | 2.764 |
| liquid/lfm-2.5-2.6b:free | 3/6 | 6/6 | 1.262 |
| poolside/laguna-xs-2.1:free | 3/6 | 3/6 | 8.287 |

Gemma's failure was Markdown framing on the precedence task. Liquid used fences
on two tasks and changed the literal tag content on another. Apodex exhausted
the output cap on precedence and returned unparsable literal-tag output. Poolside
returned HTTP 429 on three tasks; these are availability failures, not demonstrated
wrong answers. No retries concealed any failures.

Twenty complete replies supplied usage: 1375 input tokens + 7274 completion
tokens = **8649 reported processed tokens**. Completion includes 5840 reported
reasoning tokens; do not add them again. Returned cache categories were zero.
All 20 reported costs were zero. The four failed attempts did not provide captured
usable usage; their accounting remains unknown. These counts describe provider
metadata, not this Codex chat's token use or independently verified invoices.

## Decision

Keep production defaults unchanged. Gemma is the best candidate in this narrow
batch and is pinned in the free smoke-test/example manifest; validate schemas
locally and check exit/status, since prompts alone did not enforce JSON reliably.
Do not substitute free router models silently for paid/direct-provider defaults.
Future model tests must use verified free IDs or local mocks, with no paid fallback.

Reproduction/evidence: [script](evaluate_free.py), [raw results/settings/usage](free-evaluation.json),
[public catalog snapshot](free-model-catalog.json). Rerunning the script makes up
to 24 free calls and overwrites its output file; use --output for a new run.
Pricing/access can change; the script fails closed on missing/nonzero catalog
prices. [OpenRouter model catalog](https://openrouter.ai/api/v1/models) is the
source of the preflight metadata; actual returned models are recorded per response.
