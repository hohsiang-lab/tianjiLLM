# Research: OpenRouter Pricing Sync

## Decision 1: OpenRouter API Format

**Decision**: Use `GET https://openrouter.ai/api/v1/models` as secondary pricing source.

**Rationale**: 
- Free, no API key required
- Contains models missing from LiteLLM (e.g., `gemini-2.5-pro-preview` without date suffix)
- Pricing fields are per-token costs as strings — simple to parse

**Alternatives Considered**:
- Prefix/fuzzy matching on LiteLLM data → More complex, maintenance burden
- Manual JSON entries → Not scalable, overwritten on sync

**Source**: Live API query confirmed 2025-06-28

## Decision 2: Merge Strategy

**Decision**: LiteLLM-first, OpenRouter supplements. Single transaction for all upserts.

**Rationale**: 
- LiteLLM has richer metadata (max_tokens, mode, provider info)
- OpenRouter only provides pricing (prompt/completion costs)
- Using `ON CONFLICT DO UPDATE` means order matters — LiteLLM entries go first, OpenRouter only adds new model_names

**Alternatives Considered**:
- Separate transactions → Risk of partial state if second fails
- OpenRouter-first → Would lose LiteLLM's richer metadata

## Decision 3: Dual Key Storage

**Decision**: Store both `provider/model` and `model` (bare name) for each OpenRouter entry.

**Rationale**: LLM proxies may send model names in either format. Both must resolve to pricing data.

**Source**: Existing LiteLLM JSON already uses both patterns (e.g., `gemini/gemini-2.5-pro` and `gemini-2.5-pro`).

## Decision 4: Zero Pricing Handling

**Decision**: Include models with `"0"` pricing (valid free-tier models). Skip only when pricing fields are empty, null, or unparseable.

**Rationale**: Some OpenRouter models are genuinely free. Treating `"0"` as invalid would lose those entries.
