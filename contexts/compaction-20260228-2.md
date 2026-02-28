# Session Compaction Summary

## User Intent
- Improve cercle based on a real-world agent usage report and a secondary static analysis
- Fix concrete bugs and gaps found during testing (JS symbols, null vs [], HTML escaping, stale embeddings)
- Add missing features to make the RLM feedback loop complete (chunking, summary lifecycle, gemini subagent)
- Expand test coverage from zero to comprehensive

## Contextual Work Summary

### Bug Fixes
- JS symbol extraction extended: `method_definition`, `variable_declarator` (arrow/function/class expressions), `export default function/class` — caught `function_expression` vs `function` node type discrepancy via probing
- `splitCamel` fixed for Go acronym patterns (`HTMLParser` → `HTML Parser`, `GetHTTPClient` → `Get HTTP Client`) — was a real semantic search quality bug
- `null → []` on all empty search results in lexical, structural, semantic, files
- FTS5 snippet markers changed from `<b>/<b>` to `**/**` — eliminates `\u003c` JSON escaping noise
- Stale embedding on re-index fixed via SQLite trigger `documents_embedding_invalidate`: fires only when `OLD.content != NEW.content`

### New Search Features
- `GET /files` endpoint + `rlm-files` script: returns all indexed file paths, used for session orientation
- `min_similarity` param on semantic search: filters low-quality results (recommended 0.5–0.6)
- `rlm-search-semantic` script updated with `min_similarity` as 3rd arg; stale Ollama comment removed

### AST-Based Chunking (#1 from analysis)
- `parent_id` column added to `documents` via schema migration (v3)
- `insertChunks` in `indexer.go`: emits one `kind='chunk'` document per extracted symbol; content is the symbol's source lines extracted using start/end line
- Chunk paths are `{file_path}::{symbol_name}@{start_line}` — human-readable and unique
- Stale chunks pruned on re-index using upsert + `DELETE WHERE parent_id=? AND path NOT IN (...)`
- Embed worker and `PendingCount` updated to skip `kind='file'` documents that have chunks — only chunks get embeddings for files with symbols; unsymbolic files (Markdown, JSON) still embed as whole documents

### Summary Lifecycle (#5 from analysis)
- `GET /summaries` endpoint + `rlm-list-summaries` script: lists summaries newest-first with tags, preview, source
- `DELETE /summary?id=N` on existing `/summary` route (dispatched via `routeSummary`) + `rlm-delete-summary` script
- `DeleteSummary` fetches doc_id then deletes the document — cascades to FTS5, embeddings, summaries row
- Addresses "context rot in the external pool" — agents can now deprecate stale summaries

### Gemini-CLI Subagent
- `rlm/agent.md` created: defines `rlm-worker` subagent with `skills: [rlm]` frontmatter
- Reframed from read-only retrieval to autonomous implementation agent: orient → implement → report
- `rlm-submit-summary` repositioned as exit signal, not just research note
- `task install` updated to copy `agent.md` to `~/.gemini/agents/rlm-worker.md`

### Testing (from zero)
- `internal/embedder/embedder_test.go`: `splitCamel`, `tokenise`, `Load` (valid/FastText header/empty/malformed), `Embed` (known tokens, unknown tokens, unit normalization, camelCase query)
- `internal/indexer/treesitter_test.go`: JS (10 patterns), Go, Python, unsupported lang, multi-method class
- `internal/indexer/indexer_test.go`: `detectLang`, `isSupportedFile`, `shouldSkipDir`, `IndexDir` end-to-end (counts, skip dirs, upsert, source tag, embedding invalidation on re-index)
- `internal/indexer/chunks_test.go`: chunk count, path format, content isolation, stale chunk pruning, IndexDir integration, no chunks for symbol-less files — **written but not yet run**
- `internal/search/*_test.go`: lexical (stemming, global, ranked order), structural (global, prefix), semantic (nil embedder, min_similarity, limit, source filter, unknown tokens), files (global, sorted, limit, summaries excluded)
- `internal/api/handlers_test.go`: all endpoints — 200/400/405/404/500 cases, health semantic flag

### Docs
- README: RLM paper (arxiv link) and Breunig article linked inline at first citation
- REFERENCE.md: full rewrite — GloVe not Ollama, `/files` endpoint, `min_similarity`, `**` snippet format, `method` kind, new summary endpoints — **still needs chunking and summary lifecycle sections**
- SKILL.md: `rlm-files` added to workflow, `rlm-submit-summary` marked mandatory — **still needs `rlm-list-summaries` and `rlm-delete-summary`**

## Files Touched

### Daemon — Core
- **internal/db/schema.go**: `parent_id` migration (v3), idempotent index creation refactor, `documents_embedding_invalidate` trigger
- **internal/indexer/indexer.go**: `insertChunks`, `extractChunkContent`, `upsertDocument` gains `parentID` param
- **internal/indexer/treesitter.go**: `method_definition`, `variable_declarator`, `splitCamel` acronym fix
- **internal/embedder/glove.go**: `Load(io.Reader)` exported, `splitCamel` fixed for acronyms
- **internal/search/lexical.go**: `**` markers, `make([]T,0)` init
- **internal/search/structural.go**: `make([]T,0)` init
- **internal/search/semantic.go**: `minSimilarity` param, `make([]T,0)` init, `PendingCount` skips chunked files
- **internal/search/embedworker.go**: skips `kind='file'` with chunks
- **internal/search/files.go**: new — `Files()` function
- **internal/search/summaries.go**: new — `ListSummaries()`, `DeleteSummary()`
- **internal/api/handlers.go**: `handleFiles`, `handleSummaries`, `handleDeleteSummary`, `routeSummary`, `queryFloat` helper
- **internal/api/server.go**: `/files`, `/summaries` routes; `/summary` → `routeSummary`

### Skill Bundle
- **rlm/SKILL.md**: `rlm-files` step added, stronger summary mandate
- **rlm/REFERENCE.md**: full rewrite
- **rlm/agent.md**: new — `rlm-worker` gemini subagent definition
- **rlm/scripts/rlm-files**: new
- **rlm/scripts/rlm-search-semantic**: `min_similarity` arg, Ollama comment removed
- **rlm/scripts/rlm-list-summaries**: new
- **rlm/scripts/rlm-delete-summary**: new

### Tests
- **internal/embedder/embedder_test.go**: new
- **internal/indexer/treesitter_test.go**: extended
- **internal/indexer/indexer_test.go**: new
- **internal/indexer/chunks_test.go**: new (not yet run)
- **internal/search/helpers_test.go**: shared test fixtures
- **internal/search/lexical_test.go**, **structural_test.go**, **semantic_test.go**, **files_test.go**: new
- **internal/api/handlers_test.go**: new

### Project
- **Taskfile.yml**: `task install` installs agent; updated output message
- **README.md**: paper/article links, gemini install section updated
- **next.md**: new — pending work tracker
