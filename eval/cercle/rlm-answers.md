# cercle codebase — eval answers (via RLM tooling)

1. The SQL trigger that invalidates stale embeddings when a document's content is updated is named `documents_embedding_invalidate`.

2. The FTS5 `snippet()` function uses empty strings (`''`) for both the start and end markers — i.e., no delimiter characters wrap matched text. The call is `snippet(documents_fts, 1, '', '', '…', 32)`.

3. The embed worker fetches **100** documents per batch (`LIMIT 100` in the `process` method).

4. The default TCP port is **7770** (default `addr` flag: `"127.0.0.1:7770"`).

5. The `-reset` flag causes cercled to wipe the database and exit immediately without starting the HTTP server.

6. The column added in schema migration version 3 (v3) is `parent_id` (`INTEGER REFERENCES documents(id) ON DELETE CASCADE`).

7. The GloVe local embedder returns the model ID string `glove-local-dim100` for 100-dimensional vectors (from `fmt.Sprintf("glove-local-dim%d", emb.Dim())`).

8. The rlm scripts read the `CERCLE_SOURCE` environment variable to determine the source namespace.

9. The two separator characters in the chunk path format are `::` (between file path and symbol name) and `@` (between symbol name and line number), e.g. `path::Name@line`.

10. The cercle skill documentation recommends `min_similarity=0.5` to suppress noise in semantic search results.

11. Two directory names the cercle indexer skips during file tree traversal: `.git` and `node_modules` (full list also includes `vendor`, `.venv`, `__pycache__`, `dist`, `build`, `.claude`, `.gemini`).

12. The Go function that deletes a summary and its underlying document is `DeleteSummary` (in `internal/search/summaries.go`).

13. The default result limit for the `GET /summaries` endpoint is **50** (`if limit <= 0 { limit = 50 }`).

14. The macOS command that must be run after building the cercled binary is `codesign -s - <binary>` (to allow it to use the SQLite extension).

15. The Go build tag required to enable FTS5 full-text search support is `sqlite_fts5`.

16. The embed worker uses a maximum of **4** concurrent goroutines (`workerConcurrency = 4`).

17. The download-vectors script fetches the `glove.6B` dataset (by filename prefix) from Stanford NLP.

18. The two JSON field names for the start and end line range in cercle's Symbol struct are `start_line` and `end_line`.

19. The Go module path providing the SQLite3 driver is `github.com/mattn/go-sqlite3`.

20. A `semantic: false` value in the `/health` response indicates that the embedder is `nil` — word vectors were not loaded, so semantic search is disabled on that daemon instance.
