---
name: rlm
description: Retrieval skill for searching indexed codebases and documents. Use when you need to find code, understand a symbol or function, search for a concept across a project, or store a summary for future retrieval. Triggers on: "search the codebase", "find where X is defined", "what does X do", "look up", "find references to", "store this summary", "remember this for later".
---

# RLM Context Retrieval

The `cercled` daemon runs locally at `127.0.0.1:7770` and exposes lexical, structural, and semantic search, a file index, and a summary write-back path. All scripts output JSON to stdout.

## Workflow

### 1. Find exact text or keywords → lexical search

```bash
./rlm/scripts/rlm-search-lexical "query string" [limit]
```

Use for: known identifiers, error messages, specific strings, log fragments.

### 2. Find code by name → structural search

```bash
./rlm/scripts/rlm-code-structure "SymbolName" [limit]
```

Use for: function names, class names, type names. Returns file path, kind, signature, and line range. Always prefer this over lexical when looking up a named symbol.

### 3. Trigger embedding after fresh indexing → embed

```bash
./rlm/scripts/rlm-embed
```

Run this after `POST /index` on a new codebase and before using semantic search. The daemon embeds asynchronously; this call wakes the worker immediately. The response includes `pending_embed` so you know how many documents are still queued.

### 4. Find by concept → semantic search

```bash
./rlm/scripts/rlm-search-semantic "concept description" [limit]
```

Use for: vague or conceptual queries where you don't know the exact name. Uses locally loaded GloVe or FastText word vectors — no external service required. Falls back gracefully if vectors are not installed (run `download-vectors` once).

### 5. Browse indexed files → file list

```bash
./rlm/scripts/rlm-files [limit]
```

Use at the start of any session to orient yourself. Returns all indexed file paths. Call this first — knowing the file tree makes all subsequent queries smarter.

### 6. Store a summary → write-back

```bash
./rlm/scripts/rlm-submit-summary "tag1,tag2" "summary text"
```

Use after completing a research subtask to persist findings. The summary becomes immediately searchable via both lexical and semantic search in future turns. This is the RLM feedback loop.

**ALWAYS submit a summary at the end of any non-trivial investigation.** A summary you write now is instantly searchable in the next session. If you skip this step, the next agent starts cold. Don't skip it.

### 7. Review stored summaries → list summaries

```bash
./rlm/scripts/rlm-list-summaries [limit]
```

Lists all summaries newest-first with tags, preview, and source. Use at the start of a session after `rlm-files` to recover prior findings without re-searching.

### 8. Delete a stale summary → deprecate

```bash
./rlm/scripts/rlm-delete-summary <id>
```

Removes a summary and its embeddings. Use when a summary describes code that has since changed. The `id` comes from `rlm-list-summaries`. Keeping stale summaries causes context rot — prune them when you know they are outdated.

## Decision tree

```
Starting a new session?
├── rlm-files (orient yourself before querying)
└── rlm-list-summaries (recover prior findings)

Just indexed a new codebase?
└── Run rlm-embed first, then wait or poll pending_embed before semantic search

Need to find something?
├── Know the exact symbol name? → rlm-code-structure
├── Know exact words it contains? → rlm-search-lexical
└── Know the concept but not the name? → rlm-search-semantic (use min_similarity=0.6 to cut noise)

Finished a research subtask?
└── ALWAYS → rlm-submit-summary (never skip this)

Summary is about code that has since changed?
└── rlm-delete-summary <id>  (prune stale summaries to prevent context rot)
```

## Error handling

- **Daemon not running**: scripts will fail with a curl error. Inform the user that `cercled` must be started (`task run` from the cercle directory).
- **No results**: broaden the query or try a different search mode.
- **Semantic search fails**: word vectors are not loaded. Run `./rlm/scripts/download-vectors` once, then restart the daemon. Fall back to lexical search in the meantime.

For endpoint details and JSON schemas, see [REFERENCE.md](REFERENCE.md).
