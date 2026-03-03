# Next Steps & Known Limitations (v2)

During real-world agent testing (Claude), several rough edges were identified in the `cercle-lite` (v2) architecture. These are documented here for future tracking and resolution.

## 1. Directory Pathing in `file-skeleton`
- **Issue**: Running `file-skeleton` on a directory silently returned nothing instead of a clear error. Agents had to manually `ls` first to find files.
- **Next Step**: Make `file-skeleton` explicitly return an error if a directory path is provided (`"Error: path is a directory, please provide a file path"`), OR alternatively, have it automatically expand horizontally into a tree-like skeleton for the entire directory.

## 2. Module-Level Scope Missing in Chunk Maps
- **Issue**: Running `read-chunk` on line 1 of files often returned `"no chunk found covering line 1"`. The heuristic chunker aggressively chunks functions and classes but does not create a fallback chunk for top-level module scope (e.g., global `const` declarations, variable instantiations, or `import`/`require` blocks).
- **Next Step**: Agents currently must fall back to standard `cat` or `head` for those lines. The indexer (`parsers.go`) should be updated to emit "gap chunks" or a universal `module-scope` chunk covering the lines between explicit functions so that 100% of the file lines are technically inside a chunk.

## 3. Lexical Context Explosion on Alternation
- **Issue**: Using `search-lexical` with `|` alternation (e.g., `generate|tint|fill`) can easily match across dozens of files. Even with the 100-line truncation cap per chunk, returning 15 matching chunks results in a 1500-line context return.
- **Next Step**: Add a global `-max-chunks` flag or default limit (e.g., max 5 chunks returned total) to `search-lexical` to prevent the agent from accidentally eating its entire context window on overly broad regex queries. 

## 4. Parser Language Boundaries
- **Issue**: The heuristic chunker currently only maps Go, Python, JS/TS, C++, CSS, and HTML. 
- **Next Step**: While sufficient for the vast majority of agent tasks, adding lightweight regex heuristics for Rust, Java, and Ruby would effectively close the gap without needing to import large `tree-sitter` grammars.
