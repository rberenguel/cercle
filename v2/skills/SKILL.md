---
name: cercle-v2
description: Code indexing and search skill using the purely local, daemonless Cercle v2 ("cercle-lite") tool. Use when you need to navigate a codebase, map out files, locate usages, or extract code blocks directly from the source using ripgrep under the hood. 
allowed-tools: Bash(cercle-lite *)
---

# Cercle v2 Context Retrieval

`cercle-lite` is a completely local, zero-dependency Go binary built around `ripgrep` (`rg`).
Output is pure, compact text—designed specifically for LLM contexts.
*Note on Reindexing: Reindexing is extremely fast. If the source code changes, you must re-run `index` instantly to avoid stale data.*

## Tools / Commands

Invoke these via `~/.cercle/cercle-lite <command> [args...]` from the root of a project.

- `index` — Traverses the codebase (respecting `.gitignore`), parses Go, Python, JS/TS, C++, CSS, HTML, and generates JSON chunk map shards mapped cleanly in `~/.cercle/indexes/`. Re-run this whenever you modify the codebase.
- `search-lexical <query>` — Full-text search using `rg`. Instead of returning single lines, this looks up the matches in the chunk map and returns the **entire function/class body** surrounding the matches. Deduplicated automatically.
- `search-fuzzy <signature>` — Fuzzy searches across the entire vocabulary of the chunk map using pure-Go Levenshtein distance. Use this if you know a symbol name but aren't sure of the exact spelling or capitalization.
- `file-skeleton <filepath>` — Returns a structural view of a specific file (just the line numbers and signatures of functions/classes/blocks). Best for quickly orienting yourself in a new file without blowing up your context window.
- `read-chunk <filepath> <line_number>` — Reads exactly the code block enclosing a specific line number. Use this after `file-skeleton` to drill down into a specific function.
- `find-usages <symbol>` — Uses `rg -w` to strictly find usages of a symbol, but maps backwards up the file hierarchy using the index to emit *only* the calling function signature names. Extreme token efficiency.
- `extract-interface <filepath>` — Extracts only the imports/includes and exported declarations (`func`, `export`, `#include`, `def`, `class`) for a file. Perfect for understanding file contracts.

## Rules

1. **Always Index First**: If the chunk maps don't exist in `~/.cercle/indexes/` or if you've recently refactored heavily, boldly run `~/.cercle/cercle-lite index`. It's incredibly fast and safe to re-run.
2. **Search-Lexical over Grep**: Prefer `search-lexical` over raw `grep` or `rg`. It gives you the full function boundaries, meaning you rarely have to follow up with `cat` to understand the context.
3. **File-Skeleton to Orient**: When you hit an unknown file, run `file-skeleton` first. Then use `read-chunk` to jump directly into the function you care about.
4. **Find-Usages for Refactoring**: Use `find-usages` when renaming or modifying a core struct. It will tell you exactly which functions in which files call it, without spamming your context with the calls themselves.
5. **Extract-Interface to Grasp Modules**: If you want to know what a module provides without reading its implementation, `extract-interface` is your best friend.

## Errors

- **Missing Index**: `open ~/.cercle/indexes/... no such file or directory`. Run `~/.cercle/cercle-lite index`.
- **Empty Results**: Be cautious with exact queries in `search-lexical`. If it's empty, try a broader term, or use `search-fuzzy` if you suspect the symbol exists but is spelled differently. 
