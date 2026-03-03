# Cercle v2 (Diet Architecture)

Cercle v2 is a purely local, daemonless, zero-dependency code indexing and search tool designed for LLM agents.

## Architecture

Unlike v1, which relies on `tree-sitter`, SQLite, and long-running daemons, v2 operates purely as a CLI utility (`cercle-lite`). It uses heuristic parsing (regex + brace counting) to understand code structures and delegates heavy lexical searching to `ripgrep` (`rg`).

### The "Chunk Map"

Instead of storing code chunks in a database, the indexer creates lightweight "Interval Maps".
These maps are JSON strings linking files and line intervals to symbols:
`{"f": "main.go", "start": 10, "end": 25, "sig": "func process()"}`

To support large codebases without memory bloat, these chunk maps are sharded globally in `~/.cercle/indexes/`.

#### The Hashing Scheme
To ensure indexes remain isolated yet deterministic regardless of the project's internal structure, Cercle v2 employs a dual-hash scheme:

1. **Workspace Hash (`<project_md5>`)**: The root directory for a project's index (`~/.cercle/indexes/<project_md5>/`) is generated using the **absolute path** of the project directory. This ensures that two projects named `backend` in different folders do not collide. A `metadata.json` is stored inside revealing the plaintext absolute path.
2. **Shard Hash (`00.json` - `ff.json`)**: The 256 individual chunk map shards are generated using the first two characters of the MD5 hash of the **relative file path** from the workspace root (e.g., `md5("internal/indexer/indexer.go")`). This ensures the shards generated for individual files remain stable regardless of where the project folder is located on the host machine.

During a search:
1. `rg` finds matches.
2. The file paths are hashed to discover their specific chunk map shard.
3. The shard is evaluated to find the exact line bounds of the surrounding function/class.
4. The requested lines are extracted directly from the source file.

## Usage

Agents can call the `cercle-lite` binary with various subcommands:

- `index`: Generates the chunk maps in `~/.cercle/`.
- `search-lexical <query>`: Full-text search returning complete function/class bodies.
- `search-fuzzy <signature>`: pure-Go Levenshtein distance search against the index to find misspelled symbols.
- `file-skeleton <filepath>`: Returns line numbers and signatures for a specific file.
- `read-chunk <filepath> <line_number>`: Returns exactly the code block enclosing a line.
- `find-usages <symbol>`: Finds usages and returns *only* the signatures of calling functions.
- `extract-interface <filepath>`: Extracts imports and exported types/functions.
