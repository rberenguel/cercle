# RLM Tooling Reflection
**Date:** 2026-03-01
**Model:** claude-sonnet-4-6
**Task:** eval/tasks/cercle/questions.md (20 questions about the cercle codebase)

---

## What was easy

- **Unique string lookups** — Questions with a distinctive value (`-reset`, `glove-local-dim100`, `sqlite_fts5`, `codesign`) resolved in a single `rlm-search-lexical` call almost every time. The BM25 ranking reliably floated the right chunk to the top.
- **Constant/flag values** — Anything defined as a Go constant, flag default, or SQL literal was trivially findable.
- **Q9** — The question itself gave the answer format (`path::Name@line`), so I just needed confirmation from the index.

---

## What was hard / required retries

- **Empty results on reasonable queries** — `rlm-search-lexical "skip directory traversal ignore"` returned nothing; I had to guess the actual tokens (`node_modules`) and retry. The tool searches for literal tokens, so conceptual synonyms don't help.
- **`rlm-code-structure`** — Returned empty for `"glove"` and `"lexicalSearch"`. It felt unreliable enough that I stopped reaching for it after two empty hits. It's unclear what it actually indexes or whether it is sensitive to casing / exact symbol spelling.
- **Exit code 22 errors** — Several parallel calls failed silently with just `Exit code 22`. No message about what went wrong (HTTP 4xx? malformed input? rate limit?). Made it hard to know whether to retry or reformulate.
- **Guessing default values** — For Q13 (the `queryInt` default for `/summaries`), the value `50` wasn't guessable without either reading the handler or finding a search that surfaced the right chunk.

## Suggested improvements

1. **Better error messages on exit code 22** — even just stderr with the HTTP status and URL would help debug failed queries fast.
2. **`rlm-code-structure` discoverability** — document what it actually indexes (function names? struct names? which symbol types?) so callers know when to reach for it vs. `rlm-search-lexical`.
3. **A filtered files listing** — something like `rlm-files "internal/search/*.go"` would help orient quickly without burning a search slot on an open-ended query.
4. **Phrase matching** — even a simple `"quoted phrase"` mode in lexical search would reduce false-positive noise on multi-word queries.

---

## Overall

The tooling felt well-designed for its purpose. `rlm-search-lexical` did ~90% of the heavy lifting; results were fast and the BM25 ranking was generally trustworthy. The main friction points were silent failures and the opacity of `rlm-code-structure`.
