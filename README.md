<div align="center">
<img src="cercle.png" alt="cercle" width="160">

# cercle

![Work in progress](https://img.shields.io/badge/status-work%20in%20progress-red)

*A faceted context engine for LLM agents.*

</div>

A compiled Go daemon that indexes codebases and documents into a multi-tier search system, exposed through thin shell scripts that any agent can call as native tools.

> Written by a combination of human asking for stuff, Gemini designing stuff, Claude writing stuff, Gemini evaluating stuff, and the human playing the telephone game.

---

<!-- vscode-markdown-toc -->
* 1. [Motivation](#Motivation)
* 2. [This is not RAG (or is it?)](#ThisisnotRAGorisit)
* 3. [Departing from the original REPL model](#DepartingfromtheoriginalREPLmodel)
* 4. [Evaluation](#Evaluation)
* 5. [Architecture](#Architecture)
	* 5.1. [The Go Daemon](#TheGoDaemon)
	* 5.2. [The Multi-Tier Data Layer](#TheMulti-TierDataLayer)
	* 5.3. [The CLI Skills](#TheCLISkills)
* 6. [Agent integration](#Agentintegration)
	* 6.1. [gemini-cli](#gemini-cli)
	* 6.2. [Claude Code](#ClaudeCode)
	* 6.3. [Source namespacing](#Sourcenamespacing)
* 7. [Quick start](#Quickstart)
* 8. [API reference](#APIreference)
* 9. [Supported languages for structural parsing](#Supportedlanguagesforstructuralparsing)
* 10. [Tradeoffs and known limitations](#Tradeoffsandknownlimitations)
	* 10.1. [Deliberate tradeoffs](#Deliberatetradeoffs)
	* 10.2. [Known limitations](#Knownlimitations)
* 11. [Acknowledgements](#Acknowledgements)

<!-- vscode-markdown-toc-config
	numbering=true
	autoSave=true
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->


##  1. <a name='Motivation'></a>Motivation

Large context windows do not solve the problem of context rot. As an agent accumulates token history — tool results, sub-agent responses, intermediate reasoning — its performance degrades. This is not a size problem; it is a *quality* problem. Filling a 1M-token window with marginally relevant content is worse than a focused 50K-token window containing only what is needed right now.

The solution is to stop treating the context window as a filing cabinet and start treating it as working memory: finite, precious, and actively managed.

This idea is formalised in the **Retrieval Language Model (RLM)** framework described by Zhang and Khattab in [*Recursive Language Models*](https://arxiv.org/abs/2512.24601v1) (MIT CSAIL, 2025): maintain two distinct context pools — a *tokenized* pool (the LLM's live context window) and a *programmatic* pool (external storage) — and let the agent control what moves between them. The agent retrieves only what it needs, when it needs it, and writes distilled summaries back to the external pool when it finishes a subtask. This creates a feedback loop: the external pool grows more useful over time as the agent annotates it with its own understanding.

Drew Breunig's [*The Potential of RLMs*](https://www.dbreunig.com/2026/02/09/the-potential-of-rlms.html) is a good accessible introduction to why this matters in practice.

---

##  2. <a name='ThisisnotRAGorisit'></a>This is not RAG (or is it?)

Retrieval-Augmented Generation (RAG) and `cercle` both involve an external store that an LLM can query. The similarity ends there.

In RAG, retrieval is something done *to* the model — the framework intercepts a query, fetches relevant documents, and injects them into the prompt. The model has no awareness of or control over this process. It cannot decide what to retrieve, when to retrieve it, or how much context to pull in. There is no write-back path: the index is built once by humans and queried passively forever. The model is a passenger.

In the RLM model, retrieval is something the model *does*. The agent decides when context is needed, chooses which search mode is appropriate for the query, and explicitly manages what enters its context window. Critically, the agent writes back: when it finishes a subtask it distils its findings into a summary and stores it in the external pool. The next session — or the next agent — starts with that accumulated understanding already indexed and searchable. The pool becomes more useful over time precisely because agents annotate it.

The other practical difference is scope. RAG pipelines are typically stateless and single-shot: retrieve, generate, discard. `cercle` is a persistent daemon shared across sessions, agents, and projects. Multiple agents can query it simultaneously, each scoped to their own namespace, and their collective write-backs compound.

If RAG is a library, `cercle` is closer to a shared second brain.

This difference shows up empirically. In a cold-session reasoning test, a vanilla agent correctly named a SQL trigger when asked about it directly — then, in a separate reasoning question, claimed that trigger did not exist and built its entire explanation around that wrong premise. The RLM-equipped agent re-retrieved the trigger definition and made it the centrepiece of the correct answer. Knowing a fact and being able to reason with it under load are not the same thing. See [Evaluation](#evaluation).

---

##  3. <a name='DepartingfromtheoriginalREPLmodel'></a>Departing from the original REPL model

The original RLM formulation proposes a Python REPL as the programmatic context pool. The agent loads data into Python variables, writes ad-hoc query code, and calls a function to trigger sub-LLM invocations. This works, but it carries a significant cost: the retrieval mechanism is *ephemeral*. Every session starts cold. The REPL holds no persistent state. Query logic is written anew each time, consuming tokens to express things that should be infrastructure.

More critically, the REPL couples retrieval power directly to the agent's ability to write correct Python against an unfamiliar data structure. The agent must understand the storage schema, the query API, and the data layout — all at query time, under context pressure.

**`cercle` replaces the REPL with a persistent, compiled daemon and a set of stateless CLI tools.**

The distinction matters for several reasons:

- **Persistence across sessions.** The daemon indexes once. Every subsequent session, across every agent, starts with a warm corpus. There is no re-ingestion cost.
- **Separation of retrieval from reasoning.** The agent does not write retrieval code. It calls a named tool with a query string. The complexity of FTS5, AST parsing, and vector similarity is fully encapsulated in the daemon.
- **Any agent, any runtime.** The interface is standard POSIX shell. Any agent that can execute a bash command — Claude Code, gemini-cli, a custom harness — speaks the same protocol without modification.
- **Security and auditability.** The agent cannot modify retrieval logic at runtime. The tools are fixed, versioned, and auditable. There is no surface for prompt injection to subvert the retrieval mechanism itself.
- **Concurrency.** Multiple agents working in parallel can query the same daemon simultaneously without conflict. The daemon handles concurrency natively in Go.

The macOS terminal *is* the REPL. It has always been.

---

##  4. <a name='Evaluation'></a>Evaluation

A manual eval over 20 codebase Q&A tasks on cercle's own source measured accuracy and context usage across two conditions: a vanilla agent answering from parametric knowledge only, and an RLM-equipped agent using the retrieval tools.

| Condition | Score | Context used |
|-----------|-------|--------------|
| Vanilla | 19.5/20 | 74% |
| RLM | 20/20 | 77% |

3% more context, half a mistake fewer. The vanilla partial miss was on a question requiring two specific internal directory names — it recalled one correct pair but missed the expected `vendor` entry.

Answers are written as markdown bullet lists rather than JSON. Switching from a JSON answer format to markdown noticeably reduced context consumption during the answer-writing step.

A second, harder question was run cold (fresh session, no prior context) to test cross-file reasoning: *"Walk through the full sequence of events that causes a chunk document to be re-embedded after its source file is modified on disk. Name each component involved."* This requires chaining facts across the indexer, the SQL trigger, and the embed worker — no single file contains the full answer.

| Condition | Correct | Context used |
|-----------|---------|--------------|
| Vanilla | ✗ | 83% available |
| RLM | ✓ | 79% available |

On raw context cost, vanilla appears to win. But its answer was substantively wrong: it claimed *"there is no automatic invalidation"* and missed the `documents_embedding_invalidate` trigger entirely — despite having named that trigger correctly in Q1. RLM retrieved the trigger definition in context and made it the centrepiece of its explanation. The 4% context premium was justified.

The pattern: RLM's overhead does not pay off for pure lookup questions on a small codebase, but it does pay off for cross-file reasoning — precisely because it re-retrieves the relevant code rather than relying on associative recall that can drop connections under reasoning load. The crossover point is codebase size and question complexity together.

The eval methodology and prompts are in [`eval/README.md`](eval/README.md). The next round of tooling improvements will address the issues raised in [`opinions/20260301-2-claude-sonnet.md`](opinions/20260301-2-claude-sonnet.md): semantic search quality, `rlm-context` usability without a prior line anchor, and lexical disambiguation when Go identifiers collide with SQL keywords.

---

##  5. <a name='Architecture'></a>Architecture

###  5.1. <a name='TheGoDaemon'></a>The Go Daemon

A lightweight, persistent background service that exposes a local REST API on `127.0.0.1:7770`. It owns all connections to the underlying databases and parsers, handles concurrent queries, and runs a background embedding worker that drains newly indexed documents into the vector store asynchronously.

###  5.2. <a name='TheMulti-TierDataLayer'></a>The Multi-Tier Data Layer

Three complementary search mechanisms, each suited to a different retrieval strategy:

**Lexical search** — SQLite with the FTS5 extension and a Porter stemmer. Fast, exact, effective for known identifiers, error messages, and specific strings.

**Structural search** — Tree-sitter parses Go, Python, and JavaScript source files into Abstract Syntax Trees. Functions, methods, classes, and types are extracted and stored relationally. Lookup by symbol name with prefix matching.

**Semantic search** — Dense vector embeddings from pre-trained static word vectors (GloVe or FastText), stored as packed float32 blobs in SQLite. Document embeddings are produced by tokenising the text — with camelCase and snake_case splitting so code identifiers decompose correctly — and averaging the vectors of known tokens into a single unit-normalised vector. Similarity is cosine distance. Effective for conceptual queries where the exact name is unknown. Requires no external service: the model is a single text file downloaded once.

For source files with extractable symbols (Go, Python, JS), the indexer emits one *chunk* document per symbol rather than embedding the whole file. Chunk paths encode the symbol and start line: `path/to/file.go::FunctionName@42`. Symbol-level embeddings give semantic search sub-file granularity — results point to the specific function, not just the file.

A background embedding worker runs inside the daemon, processing un-embedded documents in bounded batches. Indexing does not block on embedding; the worker catches up asynchronously. No external service is required.

###  5.3. <a name='TheCLISkills'></a>The CLI Skills

Stateless shell scripts. Each is a thin `curl` wrapper that hits the daemon and pipes raw JSON to stdout:

| Script | Endpoint | Use when |
|---|---|---|
| `rlm-files` | `GET /files` | Session start — orient with the full file tree |
| `rlm-list-summaries` | `GET /summaries` | Session start — recover prior agent findings |
| `rlm-search-lexical` | `GET /search/lexical` | You know exact words or identifiers |
| `rlm-search-semantic` | `GET /search/semantic` | You know the concept, not the name |
| `rlm-code-structure` | `GET /search/structural` | You know the symbol name |
| `rlm-submit-summary` | `POST /summary` | You have finished a subtask |
| `rlm-delete-summary` | `DELETE /summary` | A prior summary is now stale |
| `rlm-delete-source` | `DELETE /source` | Purge all data for a namespace before re-indexing |
| `rlm-embed` | `POST /embed` | You need to prime semantic search after fresh indexing |

---

##  6. <a name='Agentintegration'></a>Agent integration

###  6.1. <a name='gemini-cli'></a>gemini-cli

`gemini-cli` supports a native subagent architecture where the main orchestrator agent can delegate to specialist agents with isolated context windows. Subagent interactions happen in entirely separate context loops — the orchestrator's token history is not polluted by the specialist's deep retrieval work.

Subagents are defined as Markdown files with YAML frontmatter in `~/.gemini/agents/` (user-level) or `.gemini/agents/` (project-level). Skills — the tools available to a subagent — are defined in `.gemini/skills/`.

The `rlm/` directory is a self-contained bundle containing the skill scripts, a `SKILL.md` manifest, and an `agent.md` subagent definition. Install everything with:

```bash
task install
```

This copies the skill to `~/.gemini/skills/rlm` and the subagent definition to `~/.gemini/agents/rlm-worker.md`. The orchestrator can delegate complete implementation tasks to `rlm-worker` — it searches the codebase, does the work, and reports back via `rlm-submit-summary`, keeping the full implementation context out of the orchestrator's window.

Enable experimental agents in your `settings.json`:

```json
"experimental": {
  "enableAgents": true
}
```

###  6.2. <a name='ClaudeCode'></a>Claude Code

Claude Code discovers skills from `~/.claude/skills/` (personal) or `.claude/skills/` (project). The same `rlm/` bundle works without modification:

```bash
task install
```

Claude reads `SKILL.md` when the skill is triggered and executes the scripts via bash. Script output enters the context window; script code does not. (`task install` copies rather than symlinks because Claude Code does not follow symlinks into skill directories.)

###  6.3. <a name='Sourcenamespacing'></a>Source namespacing

The daemon is shared. Multiple agents, multiple master LLMs, multiple projects can all write to and read from the same instance simultaneously. To prevent cross-contamination, every document and summary is tagged with a `source` identifier.

All CLI scripts read `CERCLE_SOURCE` from the environment, defaulting to `$PWD`. An agent running in `/Users/ruben/code/myproject` automatically scopes all indexing and retrieval to that path. Two agents in different directories do not see each other's data by default.

To search across all sources — useful for an orchestrator agent that wants global context — unset `CERCLE_SOURCE`:

```bash
CERCLE_SOURCE="" rlm-search-lexical "authentication"
```

---

##  7. <a name='Quickstart'></a>Quick start

**Requirements:** Go 1.21+, CGO enabled

```bash
# Download word vectors (one-time, ~330 MB)
./rlm/scripts/download-vectors          # GloVe 6B 100d, default
VECTOR_DIM=300 ./rlm/scripts/download-vectors   # 300d variant
# Or point at a FastText .vec file:
VECTOR_URL=https://… ./rlm/scripts/download-vectors

# Build
task build

# Start the daemon (loads vectors automatically from ~/.cercle/vectors.txt)
task run

# Index a project (in another terminal)
task index DIR=/path/to/your/project

# Prime embeddings for semantic search
task embed

# Search
task search-lexical Q="error handling"
task search-structural Q="IndexDir"
task search-semantic Q="vector similarity scoring"
```

The daemon stores its database at `~/.cercle/cercle.db`. It is safe to restart; the index persists.

A minimal web UI is available at `http://127.0.0.1:7770/ui/` for verifying the daemon is running and running manual searches across all three search modes.

---

##  8. <a name='APIreference'></a>API reference

| Method | Path | Description |
|---|---|---|
| `POST` | `/index` | Index a directory. Body: `{"path": "...", "source": "..."}` |
| `GET` | `/files` | All indexed file paths. Params: `source`, `limit` |
| `GET` | `/search/lexical` | FTS5 search. Params: `q`, `source`, `limit` |
| `GET` | `/search/semantic` | Vector search. Params: `q`, `source`, `limit`, `min_similarity` |
| `GET` | `/search/structural` | Symbol lookup. Params: `q`, `source`, `limit` |
| `POST` | `/summary` | Ingest agent summary. Body: `{"tags": "...", "text": "...", "source": "..."}` |
| `GET` | `/summaries` | List summaries newest-first. Params: `source`, `limit` |
| `DELETE` | `/summary` | Delete a summary by id. Params: `id` |
| `DELETE` | `/source` | Purge all data for a namespace. Params: `source` |
| `POST` | `/embed` | Trigger embedding worker manually |
| `GET` | `/health` | Liveness check |

Full request/response schemas: [`rlm/REFERENCE.md`](rlm/REFERENCE.md).

---

##  9. <a name='Supportedlanguagesforstructuralparsing'></a>Supported languages for structural parsing

Go, Python, JavaScript/TypeScript. All other text-based files (Markdown, JSON, YAML, shell scripts, TOML) are indexed for lexical search only.

---

##  10. <a name='Tradeoffsandknownlimitations'></a>Tradeoffs and known limitations

###  10.1. <a name='Deliberatetradeoffs'></a>Deliberate tradeoffs

**Bag-of-words semantic search.** Embeddings are produced by tokenising text and averaging pre-trained static word vectors. Word order and deep contextual meaning are lost — "handle error" and "error handle" produce the same vector. This is a conscious tradeoff: no GPU, no external embedding service, no network dependency, startup in under a second. For code retrieval the loss is acceptable because identifiers and type names carry most of the signal anyway.

**Unauthenticated local API.** The daemon binds to `127.0.0.1:7770` with no authentication. Any process on the machine can query the index, read summaries, or submit new ones. This is standard practice for local developer tooling and is consistent with how language servers, dev servers, and debug adapters work. If multi-user or remote deployment is ever needed this will need revisiting.

**CGO dependency.** Both `mattn/go-sqlite3` (FTS5, WAL) and `smacker/go-tree-sitter` (AST parsing) require CGO. This means a C compiler must be present at build time and cross-compilation is not straightforward. The alternative — a pure-Go SQLite driver — lacks FTS5 support; the alternative to go-tree-sitter does not exist. The tradeoff is accepted.

###  10.2. <a name='Knownlimitations'></a>Known limitations

**No file system watching.** Re-indexing must be triggered manually (`POST /index` or `task index`). In agent workflows this is a non-issue: the orchestrator calls `POST /index` before handing off to a subagent, so the subagent always sees a current index. For interactive use the index drifts between manual runs.

**Synchronous vector loading at startup.** The daemon loads the full GloVe/FastText file into memory before the HTTP server starts accepting connections. For the default 100d file this takes roughly one second and uses ~200 MB of RAM; the 300d file takes longer and uses ~600 MB. The server is unavailable during this window. Lazy loading or a faster binary format would fix this.

To wipe the database and start fresh: `task reset` kills the daemon, deletes `~/.cercle/cercle.db`, and rebuilds. Run `task run` afterwards.

**No call graph or cross-reference index.** Structural search stores symbol declarations only. There is no way to query "where is this function called?" — that requires a cross-reference pass that Tree-sitter alone does not provide. Agents needing call-site information must fall back to lexical search.

---

##  11. <a name='Acknowledgements'></a>Acknowledgements

- [SQLite FTS5](https://www.sqlite.org/fts5.html) — full-text search engine powering lexical search
- [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) — incremental parsing library powering structural search
- [GloVe](https://nlp.stanford.edu/projects/glove/) — pre-trained word vectors powering semantic search
- [Nano Banana Pro 2](https://gemini.google.com) — the ensō icon
- [Claude](https://claude.ai) — wrote most of the code; also found most of the bugs (in a separate session, using the tool on itself)
- [Gemini](https://gemini.google.com) — designed the architecture and reviewed the code
