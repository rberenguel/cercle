# cercle codebase — vanilla answers

1. `documents_embedding_invalidate`
2. Empty strings — both the start and end match markers are `''` (no delimiter characters); matched tokens are not visually wrapped
3. `100`
4. `7770` (listens on `127.0.0.1:7770`)
5. `-reset`
6. `parent_id` (`INTEGER REFERENCES documents(id) ON DELETE CASCADE`)
7. `glove-local-dim100`
8. `CERCLE_SOURCE`
9. `::` and `@`
10. `0.5`
11. `.git` and `node_modules` (others: `vendor`, `.venv`, `__pycache__`, `dist`, `build`, `.claude`, `.gemini`)
12. `DeleteSummary`
13. `50`
14. `codesign -s - <binary>`
15. `sqlite_fts5`
16. `4`
17. `glove.6B`
18. `start_line` and `end_line`
19. `github.com/mattn/go-sqlite3`
20. The word vectors file is not loaded — the embedder is `nil` and semantic search is disabled
