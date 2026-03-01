# RLM Tooling Reflection
**Date:** 2026-03-01
**Model:** claude-sonnet-4-6
**Task:** eval/cercle/questions.md (20 questions about the cercle codebase)

---

## What was easy

- **Exact token lookups** — Questions with a known literal value (`7770`, `CERCLE_SOURCE`, `workerConcurrency`, `codesign`, `mattn`) resolved in one `rlm-search-lexical` call. BM25 ranking surfaced the right chunk reliably.
- **`rlm-read-symbol` + `rlm-context`** — Once I had a file path from any prior result, these tools were excellent. `rlm-context` on `schema.go` line 1 gave me the full schema in one shot and answered Q1, Q2, and Q6 simultaneously. Very high information density per call.
- **`rlm-code-structure`** — Worked well this session for `Lexical`, `Run`, and `process`. Exact method names ranked first as documented.
- **Constants and defaults** — `workerConcurrency`, `LIMIT 100`, `limit = 50` were all surfaced cleanly through structural/read-symbol calls with no ambiguity.

---

## What was hard / required retries

- **Q1 (trigger name)** — Searching for `"trigger"` lexically returned the Go `Trigger()` method on `EmbedWorker`, not the SQL trigger. I had to reformulate to `"CREATE TRIGGER"` to get the schema. The name collision between Go method names and SQL keywords is an inherent hazard with pure token search.
- **Q3 (embed worker LIMIT)** — Semantic search for the concept "embed worker batch size SQL LIMIT" returned UI code and file listing handlers — completely wrong domain. Only resolved by reading the `process` method body directly. Semantic search was consistently the least useful tool in this session.
- **Q7 (GloVe model ID string)** — The semantic query for "GloVe local embedder model ID" returned the `Symbol` struct and `SymbolResult` type instead. The answer was one hop away (search for `ModelID` lexically), but the semantic miss was surprising given the query was precise.
- **Schema content is indexed as one large string** — `schema.go` contains the SQL as a Go string literal. Queries like `"migration v3"` or `"invalidate embedding trigger"` don't match because those are comment tokens interleaved with SQL. You need to know to search for the SQL keyword literally (`"CREATE TRIGGER"`) rather than the concept.

---

## Suggested improvements

1. **`rlm-context` with a glob** — being able to say "give me lines 80–180 of `schema.go`" without first needing a line number from another result would save one round-trip. Currently you need a prior hit to get a line anchor.
2. **Semantic search quality** — In this session semantic was the weakest tool and I largely abandoned it after two misses. It may be a vectors-quality issue, but queries with clear technical terms (function names, SQL keywords) should probably fall back to lexical internally rather than surfacing distant semantic neighbours.
3. **Disambiguation on name collisions** — When a Go identifier (`Trigger`) and a SQL object (`CREATE TRIGGER`) share a token, there's no way to scope the search. A `kind:` filter (e.g., `kind:sql`) or file-scoped search (`rlm-search-lexical "CREATE TRIGGER" --file "*.go"`) would help.
4. **`rlm-list-summaries` at session start** — The SKILL.md says to do this when context is cold, but I skipped it. Worth making it more prominent or automatically printing a count so agents know whether prior research exists.

---

## Overall

The tooling held up well under a systematic 20-question eval. `rlm-read-symbol` and `rlm-context` were the MVPs — once you have a foothold in the right file, you can extract almost anything with a single follow-up call. Lexical search is reliable for known tokens. The weak spots are semantic search accuracy and conceptual queries that don't map to verbatim source tokens.
