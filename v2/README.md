> [!CAUTION]
> This has moved to [garbell](https://github.com/rberenguel/garbell)

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

Each command answers a distinct question about a codebase:

| Question | Command |
|---|---|
| *What exists and where?* | `file-skeleton <path>` |
| *What does this do?* | `read-chunk <file> <line>` |
| *Where is X mentioned?* | `search-lexical <query>` |
| *Who calls this?* | `find-usages <symbol>` |
| *What does this expose?* | `extract-interface <file>` |
| *What has this shape/signature?* | `search-signature <pattern>` |
| *Where is the complexity?* | `largest-chunks [n]` |
| *What does this call?* | `callees <file> <line>` |
| *What imports this?* | `dependents <file>` |

Full command reference: [`REFERENCE.md`](REFERENCE.md).

## Test with a specific codebase

During a real-world test on a complex ~1300 line JS codebase ([destrier](https://github.com/rberenguel/destrier)), Claude provided excellent feedback on `cercle-lite`.

For context, the LLM was given the following ambitious prompt to implement a two-player versus mode:

> I want you to create a very ambitious new extension for this game: two player versus mode. Make sure to read the @README.md completely
>
> Two player vs needs new menu selection, needs configurable controls for the second player, handling of more than 1 external controller via web bluetooth (currently we restrict to 1 IIRC), a different way of handling "waves", smart positioning of the two ships, coloring each differently… Several things.
>
> This is hard, and I want to see how well you handle this (which I have delayed for almost a year!)

The workspace consisted of actually over 60 files and roughly 134,000 lines of code (including the `lib/` folder, as `cercle-lite` indexes all non-gitignored files):

```text
--------------------------------------------------------------------------------
 Language             Files        Lines        Blank      Comment         Code
--------------------------------------------------------------------------------
 JavaScript              54       132623         1751         8510       122362
 CSS                      4         1090          126           21          943
 HTML                     3          200            1            3          196
 Markdown                 2          158           49            0          109
 Plain Text               1           93           18            0           75
 JSON                     1           35            0            0           35
 Bourne Shell             1           21            6            1           14
--------------------------------------------------------------------------------
 Total                   66       134220         1951         8535       123734
--------------------------------------------------------------------------------
```

Claude provided the following "testimonial" regarding the efficiency of `cercle-lite` for this task:

> **What worked great:**
> - `file-skeleton` was the killer feature. Getting just function signatures and line ranges for a 1300-line file like destrier.js or scene.js in one call, then drilling in with `read-chunk`, was much more efficient than reading whole files.
> - `search-lexical` returning full function bodies around matches (not just lines) meant I rarely needed follow-up reads.
> - The `find-usages` → `file-skeleton` → `read-chunk` workflow felt natural and fast.
>
> **Bottom line:** For a codebase of this size (~60 files), it probably saved 30-40% context vs naive file reading. The `file-skeleton` → `read-chunk` pattern is particularly strong.

### Context Retention Metrics

To quantify the context savings, the remaining context window capacity was measured at two critical points during the agent's lifetime: just after asking clarifying questions (exploring the repo) and just after requesting plan approval.

| Tooling Approach     | Context Remaining (Clarifying Questions) | Context Remaining (Plan Approval) |
| :---                 | :---:                                    | :---:                             |
| **Cercle v1**        | 63%                                      | 48%                               |
| **Cercle v2 (lite)** | 59%                                      | 47%                               |
| **Vanilla**          | 41%                                      | 38%                               |

*Note: Vanilla refers to the agent relying solely on native "read file" tools. Both Cercle v1 and v2 effectively preserved ~20% more context capacity heading into the design phase than native tools.*
