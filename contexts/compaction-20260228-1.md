# Session Compaction Summary

## User Intent
- Build a local RLM (Retrieval Language Model) context engine to fight context rot in LLM agents
- Replace the REPL-based approach from the original RLM paper with a persistent Go daemon + stateless CLI tools
- Make it work as a skill for both Claude Code and gemini-cli
- Keep everything local: no cloud services, no Ollama, GloVe word vectors instead

## Contextual Work Summary

### Go Daemon (cercled)
- Persistent HTTP server on `127.0.0.1:7770`, SQLite backend with WAL mode
- Three-tier search: FTS5 lexical, Tree-sitter structural (Go/Python/JS), GloVe semantic
- Background `EmbedWorker` goroutine drains un-embedded documents asynchronously (batch of 100, concurrency 4)
- Version stamped at build time via `-ldflags "-X main.Version={{.VERSION}}"` from `VERSION` file
- Binary requires ad-hoc codesigning on macOS (`codesign -s -`) and `sqlite_fts5` build tag
- Daemon starts without vectors; semantic search gracefully disabled until `download-vectors` is run

### SQLite Schema
- `documents` table: path, kind (file/summary/chunk), content, lang, **source** (namespace tag), indexed_at
- FTS5 virtual table with triggers keeping it in sync with `documents`
- `symbols` table: Tree-sitter extracted functions/classes/types with line ranges
- `embeddings` table: packed float32 blobs, keyed by `(doc_id, model)`
- `summaries` table: agent write-back with tags
- Additive migration system via `ALTER TABLE` with duplicate-column error ignored

### Source Namespacing
- Every document/summary tagged with `source` (defaults to `$PWD` via `CERCLE_SOURCE` env var)
- All search endpoints filter by `source` when provided; omit for cross-agent global search
- Prevents multiple agents in different projects from polluting each other's results

### GloVe Embedder
- `internal/embedder/glove.go`: loads GloVe or FastText text-format vector files into memory
- Code-aware tokenisation: splits camelCase (`IndexDir` → `index dir`), snake_case, strips short tokens
- Embeddings are averaged token vectors, unit-normalised
- Model ID derived from dimensionality (`glove-local-dim100`) for DB consistency
- `rlm/scripts/download-vectors`: fetches `glove.6B.zip` from Stanford, extracts 100d file to `~/.cercle/vectors.txt`

### RLM Skill Bundle (`rlm/`)
- `SKILL.md`: frontmatter with `name`/`description` only (per spec), decision tree workflow, error handling
- `REFERENCE.md`: full API reference for all endpoints, JSON schemas, supported file types
- `scripts/`: `rlm-search-lexical`, `rlm-search-semantic`, `rlm-code-structure`, `rlm-submit-summary`, `rlm-embed`, `download-vectors`
- All scripts default `CERCLE_SOURCE=$PWD`, `CERCLE_ADDR=127.0.0.1:7770`

### Skill Installation
- `task install` copies `rlm/` to `~/.claude/skills/rlm` and `~/.gemini/skills/rlm` (symlinks rejected by Claude Code)

## Files Touched

### Daemon
- **cmd/cercled/main.go**: entrypoint, flag parsing, vector loading, worker startup, version var
- **internal/db/db.go**: SQLite open with WAL + foreign keys
- **internal/db/schema.go**: full schema + additive ALTER TABLE migration system
- **internal/indexer/indexer.go**: directory walker, upsert with source tag
- **internal/indexer/treesitter.go**: AST symbol extraction for Go/Python/JS
- **internal/search/lexical.go**: FTS5 MATCH with source filter
- **internal/search/structural.go**: symbol prefix search with source filter
- **internal/search/semantic.go**: cosine similarity over stored vectors, uses Embedder
- **internal/search/embedworker.go**: background goroutine, semaphore-bounded Ollama→GloVe replacement
- **internal/embedder/glove.go**: GloVe/FastText loader, code-aware tokeniser, averaging embedder
- **internal/api/server.go**: HTTP mux, routes, version in /health
- **internal/api/handlers.go**: all request handlers, source threading

### Skill
- **rlm/SKILL.md**: skill manifest and workflow instructions
- **rlm/REFERENCE.md**: API reference (Level 3 resource, loaded on demand)
- **rlm/scripts/***: 6 executable bash scripts

### Project
- **go.mod**: module `github.com/ruben/cercle`, go 1.21, two CGO deps
- **Taskfile.yml**: build (fts5 tag + codesign + ldflags), run, index, search-*, embed, install, clean
- **VERSION**: `0.0.1`
- **README.md**: architecture, RLM motivation, REPL departure rationale, quick start, API table
- **.gitignore**: bin/, *.db*, vectors.txt

## Next Steps
- Complete first real test: index `../destrier`, run `task embed`, open Claude session in destrier
- Verify all three search modes return results
- Verify `rlm-submit-summary` write-back is retrievable in subsequent searches
