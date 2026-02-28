# Next steps

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
