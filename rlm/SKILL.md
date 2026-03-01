---
name: rlm
description: Retrieval skill for searching indexed codebases and documents. Use when you need to find code, understand a symbol or function, search for a concept across a project, or store a summary for future retrieval. Triggers this with 'search the codebase", 'find where X is defined', 'what does X do', 'look up', 'find references to', 'store this summary'.
allowed-tools: Bash(rlm-* *)
---

# RLM Context Retrieval

`cercled` at `127.0.0.1:7770`. Scripts output markdown (formatted by [`rlm-format`](scripts/rlm-format)). All scoped to `CERCLE_SOURCE` (defaults to `$PWD`).

## Tools

- [`rlm-code-structure`](scripts/rlm-code-structure) `"fragment" [n]` — Go/Py/JS functions, methods, types — NOT constants. Exact name ranks first. Preview: 20 lines.
- [`rlm-search-semantic`](scripts/rlm-search-semantic) `"concept" 2 0.5` — Always pass limit=2 min_similarity=0.5. Returns 800-char snippet. Scores are not quality rankings — read the snippet.
- [`rlm-search-lexical`](scripts/rlm-search-lexical) `"token" [n]` — Exact verbatim tokens only. ANDs all tokens. Returns {results, query} — check query to see what was searched.
- [`rlm-read-symbol`](scripts/rlm-read-symbol) `<path> <symbol>` — Full body of a symbol from a prior structural result.
- [`rlm-context`](scripts/rlm-context) `<path> <line> [n]` — n lines around a line number from any result. Use for constants and file-scope content.
- [`rlm-files`](scripts/rlm-files) `["*/glob"] [n]` — File list. Use when orienting in an unfamiliar codebase.
- [`rlm-list-summaries`](scripts/rlm-list-summaries) `[n]` — Prior agent findings. Use at session start when context is cold.
- [`rlm-submit-summary`](scripts/rlm-submit-summary) `"tags" "text"` — Persist findings. Always do after non-trivial research.
- [`rlm-embed`](scripts/rlm-embed) — Wake embed worker after fresh indexing.
- [`rlm-delete-summary`](scripts/rlm-delete-summary) `<id>` — Remove a stale summary.
- [`rlm-index`](scripts/rlm-index) `<path> [source]` — Index a directory and wait for embedding.
- [`rlm-delete-source`](scripts/rlm-delete-source) `[source]` — Remove all documents for a source namespace. Use before re-indexing heavily refactored projects.

## Rules

1. **Structural first** when you can guess any fragment of a function, method, or type name. Exact names rank first.
2. **Semantic** for everything else — concepts, "what is X named", "how does Y work". Always `limit=2 min_similarity=0.5`.
3. **Lexical** only for strings you know appear verbatim in source. Empty result → switch to semantic immediately, no retry.
4. **rlm-read-symbol** when a structural preview cuts off before the answer.
5. **rlm-context** when you have a file path and line number but need surrounding content (e.g. constants).

## Errors

- **Daemon not running**: scripts fail with connection error. Run `task run`.
- **Empty structural results**: symbol not in Go/Py/JS or named differently. Use semantic.
- **Semantic error**: vectors not loaded. Run `./rlm/scripts/download-vectors`, restart daemon. Use lexical meanwhile.

For endpoint schemas: [REFERENCE.md](references/REFERENCE.md).
