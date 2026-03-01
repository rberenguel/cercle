# cercle codebase — ground truth answers

1. `documents_embedding_invalidate` — defined in `internal/db/schema.go`, fires `AFTER UPDATE OF content ON documents` when `OLD.content != NEW.content`, deletes rows from `embeddings` for that document so the embed worker re-processes it.

2. `''` (empty string) — used as both the start and end highlight marker in the `snippet()` call in `internal/search/lexical.go`: `snippet(documents_fts, 1, '', '', '…', 32)`.

3. `100` — the embed worker queries `LIMIT 100` pending documents per batch in `internal/search/embedworker.go`.

4. `7770` — the default listen address is `127.0.0.1:7770`, set in `cmd/cercled/main.go`.

5. `-reset` — the flag is defined as `flag.Bool("reset", false, "Wipe the database and exit")` in `cmd/cercled/main.go`; it deletes the `.db`, `.db-shm`, and `.db-wal` files then exits before the server starts.

6. `parent_id` — added as `ALTER TABLE documents ADD COLUMN parent_id INTEGER REFERENCES documents(id) ON DELETE CASCADE` in `internal/db/schema.go`; links chunk documents back to their parent file.

7. `glove-local-dim100` — returned by `ModelID()` in `internal/search/semantic.go` as `fmt.Sprintf("glove-local-dim%d", emb.Dim())` for a 100-dimensional GloVe model.

8. `CERCLE_SOURCE` — read at the top of every rlm script as `CERCLE_SOURCE="${CERCLE_SOURCE:-$PWD}"`, scopes all searches and indexing to a project namespace.

9. `::` and `@` — the chunk path format is `{filepath}::{SymbolName}@{startLine}`, constructed in `internal/indexer/indexer.go` as `fmt.Sprintf("%s::%s@%d", filePath, sym.Name, sym.StartLine)`.

10. `0.5` (or `0.5–0.6`) — recommended in `rlm/SKILL.md` as `min_similarity=0.5` and the `rlm-search-semantic` script header: "Use 0.5–0.6 to suppress noise".

11. `node_modules` and `vendor` — among the directories skipped in the walk in `internal/indexer/indexer.go`; the full skip list also includes `.git`, `.venv`, `__pycache__`, `dist`, `build`.

12. `DeleteSummary` — defined in `internal/search/summaries.go`, removes a summary row and its underlying document (cascading to FTS5 and embeddings).

13. `50` — the default passed to `queryInt(r, "limit", 50)` in the `/summaries` handler in `internal/api/handlers.go`.

14. `codesign` — run as `codesign -s - ./bin/cercled` in `Taskfile.yml` after the build step; required on macOS to allow the CGO SQLite binary to run without Gatekeeper issues.

15. `sqlite_fts5` — passed via `-tags "sqlite_fts5"` in `go build` and `go test` commands in `Taskfile.yml`; enables the FTS5 extension in the `go-sqlite3` CGO build.

16. `4` — defined as `const workerConcurrency = 4` in `internal/search/embedworker.go`; controls the semaphore size used to limit concurrent embedding goroutines.

17. `glove.6B` — the download-vectors script fetches `https://nlp.stanford.edu/data/glove.6B.zip` and extracts `glove.6B.{DIM}d.txt` from it.

18. `start_line` and `end_line` — the JSON tags on `StartLine int` and `EndLine int` fields of the `Symbol` struct in `internal/search/structural.go`.

19. `github.com/mattn/go-sqlite3` — listed in `go.mod` as `github.com/mattn/go-sqlite3 v1.14.24`, imported as a blank import in `internal/db/db.go`.

20. Word vectors are not loaded — `semantic: false` means the `Embedder` is nil (no vectors file was found or loaded at startup), so semantic search is disabled and calls to `/search/semantic` will return an error.
