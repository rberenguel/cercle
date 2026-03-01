# Session Compaction Summary

## User Intent
- Add user-configurable file/directory exclusion via `.cercleignore` and `.gitignore`
- Build a local browser UI for the cercle daemon (developer-facing, not agent-facing)
- Fix correctness bugs in embedding pipeline (slowness, miscounted pending)

## Contextual Work Summary

### Ignore File Support
- New `IgnoreList` type in `internal/indexer/ignore.go` with gitignore-subset semantics: basename patterns, anchored patterns (containing `/`), directory-only patterns (trailing `/`), comments/blanks ignored
- `LoadIgnoreFiles` loads `.cercleignore` then `.gitignore` from the indexed root (both optional)
- `IndexDir` now loads ignore files before walking and applies `ShouldIgnore` for both dirs (`SkipDir`) and files (`Skipped++`)
- `.cercleignore` added at repo root: ignores `eval/`, `opinions/`, `contexts/`
- Full unit + integration test suite in `internal/indexer/ignore_test.go`
- `rlm/REFERENCE.md` updated with ignore file documentation

### Web UI
- Self-contained UI at `internal/api/ui/` (embedded into binary via `//go:embed`)
- Served by the daemon at `/ui/` — no CORS issues, no separate process
- Three tabs: **Search** (lexical/structural/semantic, all three in parallel columns), **Files**, **Summaries**
- **Index tab** added: text input for path + optional source tag, fires `POST /index`, shows files/symbols/skipped/pending counts; lists currently indexed sources below form (click to prefill path)
- Solarized dark theme, Monoid font (drop files into `internal/api/ui/fonts/`)
- Daemon startup log now prints clickable UI URL

### New `/sources` Endpoint
- `GET /sources` returns per-namespace stats: files, chunks, embedded, pending
- Used by index tab (on open) and files tab (grouped view when no source filter)
- Files tab groups results under cyan source headers with stats; flat when source filter active

### Embedding Fixes
- **Slowness**: `EmbedWorker.process` returned a bool; `Run` now loops until queue is drained instead of waiting 30s between batches
- **Phantom pending (parent files)**: `Sources` query now mirrors `PendingCount`'s exclusion of parent files (files that have chunks — intentionally never embedded directly)
- **Phantom pending (unencodable docs)**: `EmbedAndStore` now always writes a row — empty-blob sentinel for nil vectors — so the worker never revisits them and they stop counting as pending
- Semantic search queries filter `length(e.vector) > 0` to exclude sentinel rows from results

## Files Touched

### Core Logic
- **`internal/indexer/ignore.go`**: New — `IgnoreList`, `LoadIgnoreFile`, `ShouldIgnore`, `LoadIgnoreFiles`
- **`internal/indexer/indexer.go`**: Load ignore files in `IndexDir`, apply in walk callback
- **`internal/search/files.go`**: New `Sources()` function + `SourceInfo` struct; `Sources` query fixed for accurate embedded/pending counts; `Files` relativizes paths
- **`internal/search/semantic.go`**: `EmbedAndStore` writes sentinel for nil vecs; `Semantic` excludes sentinels; path relativization + chunk-parent suppression added
- **`internal/search/embedworker.go`**: `process` returns bool; `Run` loops until drained

### API
- **`internal/api/handlers.go`**: New `handleSources` handler
- **`internal/api/server.go`**: `//go:embed ui`, serves `/ui/`, registers `/sources` route
- **`cmd/cercled/main.go`**: Startup log includes UI URL

### UI
- **`internal/api/ui/index.html`**: Full page structure — search, files, summaries, index tabs
- **`internal/api/ui/app.js`**: All tab logic, API calls, rendering; relative URLs (same-origin)
- **`internal/api/ui/style.css`**: Solarized dark, Monoid, source group headers, index tab stats
- **`internal/api/ui/fonts/.gitkeep`**: Placeholder for user-provided font files

### Config & Tests
- **`.cercleignore`**: `eval/`, `opinions/`, `contexts/`
- **`internal/indexer/ignore_test.go`**: Unit + integration tests for ignore logic
