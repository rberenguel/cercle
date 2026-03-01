# RLM Daemon API Reference

Base URL: `http://127.0.0.1:7770`
Override with env var: `CERCLE_ADDR`

---

## POST /index

Index a directory of files into the daemon.

**Request**
```json
{ "path": "/absolute/or/relative/path" }
```

**Response**
```json
{ "files": 42, "symbols": 318, "skipped": 5, "errors": [], "pending_embed": 42 }
```

---

## GET /files

Return indexed file paths. Use this at the start of a session to orient yourself.

**Query params**
| Param    | Required | Default | Description |
|----------|----------|---------|-------------|
| `source` | no       | —       | Filter to this source namespace |
| `glob`   | no       | —       | SQLite GLOB pattern to filter paths (e.g. `*/internal/*.go`) |
| `limit`  | no       | 0       | Max results (0 = no limit) |

**Response**
```json
{ "files": ["internal/a.js", "internal/b.go"] }
```

Paths are relative to `source` when a source filter is provided; absolute otherwise.
GLOB uses `*` (any sequence) and `?` (single character). Case-sensitive.

---

## GET /search/lexical

Full-text search via SQLite FTS5 (porter stemmer).

**Query params**
| Param    | Required | Default | Description |
|----------|----------|---------|-------------|
| `q`      | yes      | —       | Search query |
| `source` | no       | —       | Filter to this source namespace |
| `limit`  | no       | 10      | Max results |
| `raw`    | no       | `0`     | Set to `1` to pass `q` directly to FTS5 without sanitization |

**Default mode** (`raw=0`): the query is sanitized — non-alphanumeric characters (`.`, `-`, `*`, `"`, etc.) are replaced with spaces, producing a keyword AND search. This prevents FTS5 syntax errors from natural-language input like `os.Exit` or `command-line`.

**Raw mode** (`raw=1`): query is passed to FTS5 as-is. Supports full FTS5 syntax:
| Syntax          | Meaning |
|-----------------|---------|
| `foo bar`       | AND — both tokens must appear |
| `foo OR bar`    | OR — either token |
| `"foo bar"`     | Exact phrase match |
| `foo*`          | Prefix match |

For conceptual or synonym queries, use `/search/semantic` instead.

**Response**
```json
{
  "query": "sanitized tokens",
  "results": [
    { "id": 7, "path": "internal/file.go", "snippet": "…matched text…", "rank": -0.83 }
  ]
}
```
`query` shows the tokens that were AND-ed — useful for diagnosing empty results.
Paths are relative to `source` when a source filter is provided.
When both a file and one of its symbol chunks match, the parent file is suppressed.

---

## GET /search/structural

Symbol lookup by name substring (functions, classes, methods, types).
Case-insensitive for ASCII. Only covers Go, Python, and JavaScript sources.

**Query params**
| Param    | Required | Default | Description |
|----------|----------|---------|-------------|
| `q`      | yes      | —       | Symbol name substring |
| `source` | no       | —       | Filter to this source namespace |
| `limit`  | no       | 20      | Max results |

**Response** — array of:
```json
{
  "path": "internal/indexer/indexer.go",
  "kind": "function",
  "name": "IndexDir",
  "signature": "func IndexDir(ctx context.Context, db *sql.DB, root, source string) (*Result, error)",
  "start_line": 18,
  "end_line": 60,
  "preview": "func IndexDir(...) {\n\t…first 20 lines…"
}
```

`kind` values: `function`, `method`, `class`, `type`

`preview` contains the first 20 lines of the function body. Omitted when no chunk is available.
Paths are relative to `source` when a source filter is provided.

---

## GET /search/semantic

Vector similarity search using GloVe embeddings (cosine distance). No external service required.

**Query params**
| Param            | Required | Default | Description |
|------------------|----------|---------|-------------|
| `q`              | yes      | —       | Natural language concept |
| `source`         | no       | —       | Filter to this source namespace |
| `limit`          | no       | 10      | Max results |
| `min_similarity` | no       | 0       | Minimum cosine similarity (0–1). Use 0.5–0.6 to filter noise. |

**Response** — array of:
```json
{ "id": 7, "path": "internal/file.go", "kind": "file", "similarity": 0.91, "snippet": "First 800 chars of content…" }
```

Requires word vectors to be loaded (run `download-vectors` once, then restart daemon).
Only returns results for documents that have been embedded (embedding happens asynchronously after indexing).

Symbol chunks have paths in the form `internal/file.go::FunctionName@42` where `42` is the start line. The parent file itself is not embedded when chunks exist — only the chunks are.

Paths are relative to `source` when a source filter is provided. When both a file and one of its chunks score above the threshold, the parent file is suppressed.

---

## POST /summary

Write an agent-generated summary into the index. The summary is immediately searchable via lexical and semantic search.

**Request**
```json
{
  "tags": "comma,separated,tags",
  "text": "Full summary text...",
  "source": "/path/to/project"
}
```

**Response**
```json
{ "id": 99, "status": "indexed" }
```

Embedding is generated asynchronously — semantic search for this summary may lag by a few seconds.

---

## GET /summaries

List stored summaries in reverse chronological order.

**Query params**
| Param    | Required | Default | Description |
|----------|----------|---------|-------------|
| `source` | no       | —       | Filter to this source namespace |
| `limit`  | no       | 50      | Max results |

**Response** — array of:
```json
{ "id": 3, "tags": "auth,session", "created_at": 1709000000, "preview": "First 300 chars of summary text…" }
```

---

## DELETE /summary?id=N

Delete a summary by its summary `id` (from `GET /summaries`). Cascades to the underlying document, FTS5 index, and embeddings.

**Query params**
| Param | Required | Description |
|-------|----------|-------------|
| `id`  | yes      | Summary ID (integer) |

**Response**
```json
{ "id": 3, "status": "deleted" }
```

Returns 404 if the summary does not exist.

---

## DELETE /source?source=<ns>

Remove all documents, chunks, embeddings, and summaries for a source namespace. Use this before re-indexing a project that has been heavily refactored, moved, or deleted — it is cheaper than a full database wipe and leaves other namespaces untouched.

**Query params**
| Param    | Required | Description |
|----------|----------|-------------|
| `source` | yes      | Source namespace to purge (exact match) |

**Response**
```json
{ "source": "/path/to/project", "deleted": 87 }
```

`deleted` is the total number of document rows removed (files + chunks + summaries). Cascades automatically remove embeddings, symbols, and FTS5 entries.

---

## POST /embed

Trigger the background embedding worker manually. Call this after indexing a new codebase to prime semantic search.

**Response**
```json
{ "status": "triggered", "pending_embed": 42 }
```

---

## GET /health

Liveness check.

**Response**
```json
{ "status": "ok", "version": "0.0.1", "semantic": true }
```

`semantic: false` means word vectors are not loaded; semantic search will return an error.

---

## Supported file types for indexing

| Extension | Language | Symbol extraction |
|-----------|----------|------------------|
| `.go` | Go | functions, methods, types |
| `.py` | Python | functions, classes |
| `.js`, `.ts` | JavaScript | functions, methods, classes (including ES module exports and arrow functions) |
| `.md`, `.txt` | — | FTS5 only |
| `.json`, `.yaml`, `.yml`, `.toml`, `.sh` | — | FTS5 only |

## Skipped directories

`.git`, `node_modules`, `vendor`, `.venv`, `__pycache__`, `dist`, `build`

## Ignore files

Place a `.cercleignore` file in the root of the directory being indexed to exclude files or directories. The project's existing `.gitignore` is also read automatically if present.

Supported syntax (subset of gitignore):

| Pattern | Behaviour |
|---------|-----------|
| `# comment` | Ignored |
| blank line | Ignored |
| `pattern` (no `/`) | Matches basename anywhere in tree |
| `dir/pattern` (contains `/`) | Anchored to indexed root |
| `pattern/` (trailing `/`) | Directories only |
| `*`, `?`, `[…]` | filepath.Match glob semantics |

`**` and negation (`!`) are not supported in v1.

Applied after the built-in directory and extension filters.
