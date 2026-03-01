# Next steps

_Items marked **[eval]** come directly from agent evaluation feedback._

## [eval] Single-file re-index

The daemon currently only accepts a directory for `POST /index`. During an active editing session, agents
need to re-index individual files after writes — a full directory walk is too slow and too disruptive.

Two changes:
- Fix `IndexDir` to handle a file path as well as a directory (currently `LoadIgnoreFiles` would fail on a file path).
- Add `rlm-index-file <path>` script as a thin wrapper: `POST /index` with the file's path and the current source namespace.

This makes the re-index cost proportional to what changed, not to the whole codebase.

## [eval] Search-time path exclusion (default, overridable)

During the eval, large third-party JS bundles (`libs/3rdparty/Tone.js`, `metap.js`, etc.) polluted semantic
results. The right fix is **not** to skip indexing them — the agent may legitimately need to search vendor
code (debugging a third-party bug, checking an API signature). Filtering at index time removes that option
entirely and violates the RLM principle that the agent controls its own context.

Instead: add an optional `exclude` glob param to search endpoints (lexical, semantic, structural) that
defaults to common vendor path patterns when not specified.

```
GET /search/semantic?q=collision&exclude=libs/**,vendor/**,node_modules/**   (explicit)
GET /search/semantic?q=collision                                               (same default applied)
GET /search/semantic?q=collision&exclude=                                      (agent opts in to everything)
```

The default exclude list lives in the daemon config (or hardcoded initially). The agent can widen or narrow
it per-query without affecting the index. `.cercleignore` remains the right tool for "never index this",
while `exclude` is for "usually skip but I decide".

## [eval] File watcher (daemon-side auto-reindex) — low priority

Longer-term answer to the staleness problem: the daemon watches directories it has indexed and re-indexes
modified files automatically. Triggered by `fsnotify` events, batched with a short debounce (e.g. 500ms)
to avoid thrashing during rapid edits.

Likely not worth the complexity: SKILL.md guidance (see below) covers the real cases, and the agent
controlling the re-index cadence is more in the spirit of the RLM model anyway.

## [eval] SKILL.md: session phase guidance + re-index cadence

The eval confirmed the use-case split the design intended, but agents don't know to reason about it.
Add a "When to use RLM" section near the top of SKILL.md:

- **Cold-start / unfamiliar codebase**: RLM is high-value — `rlm-index .` first, use structural and semantic
  search to orient before reading any files.
- **Active editing session**: after the key files are open and understood, direct `Grep`/`Read` are faster
  and always current. Re-index when search results feel stale: after 3+ file edits, or before any structural/
  semantic search on recently modified code. `rlm-index .` polls until embedding is complete — it is safe to
  wait on it.
- **Write-back**: `rlm-submit-summary` is most valuable at natural checkpoints (end of a subsystem exploration,
  after a non-obvious decision) — not after every file read.

This makes re-indexing a deliberate, agent-driven action rather than infrastructure to build.



## Kind filter on search

Add a `kind` query param to `/search/lexical` and `/search/semantic` so agents can scope queries to a specific document kind.

```
GET /search/lexical?q=auth&kind=summary   → only search prior summaries
GET /search/lexical?q=auth&kind=file      → only search indexed files
GET /search/lexical?q=auth&kind=chunk     → only search symbol chunks
```

Lets agents distinguish "what have I previously concluded about X" from "what does the codebase say about X" — a meaningful separation in the RLM model between the agent-written layer and the source layer.

## Tag filter on /summaries

Add a `tags` query param to `GET /summaries` to filter by tag (substring or exact match TBD).

```
GET /summaries?tags=auth
GET /summaries?tags=bugfix&source=/path/to/project
```

Tags already exist on every summary row — this is purely a query addition. Makes the summary pool navigable when it grows large.

## Recency bias on semantic search

Add an optional `recency_weight` param to `/search/semantic` that blends cosine similarity with document recency (`indexed_at`). When scores are close, newer results surface first.

Motivated by the RLM paper's framing of the programmatic pool as an actively managed, time-aware store rather than a static index.
