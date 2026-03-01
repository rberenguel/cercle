# cercle eval harness

Measures the accuracy gap between vanilla LLM agents and cercle's RLM-augmented agents on codebase Q&A tasks.

The hypothesis: an agent with cercle's retrieval tools (lexical, structural, semantic search) should answer codebase-specific questions significantly better than one with no context at all.

---

## Prerequisites

1. **`cercled` is running** — `task run` from the cercle root
2. **Target codebase is indexed** — see below
3. **`claude` CLI is on your PATH** — install via `npm i -g @anthropic-ai/claude-code` or equivalent
4. **`gemini` CLI is on your PATH** — only needed if you include `gemini` in `--models`

---

## Indexing a codebase

Use the `rlm-index` script (same family as the other rlm scripts the skill uses):

```sh
# Index cercle itself
rlm/scripts/rlm-index /path/to/cercle

# Index another project (e.g. etcd)
rlm/scripts/rlm-index /path/to/etcd
```

Indexing is incremental — re-running it updates changed files. The response shows how many files, symbols, and errors were processed. The namespace defaults to the absolute path of the directory.

---

## Specifying the target codebase

The `--source` flag tells the harness **which indexed codebase to evaluate against**. Pass the same path you gave to `POST /index` (the daemon uses the absolute path as the default namespace).

```sh
# Evaluate against cercle itself
python eval/eval.py --tasks eval/tasks/cercle.json --source /path/to/cercle

# Evaluate against another project
python eval/eval.py --tasks eval/tasks/myproject.json --source /path/to/myproject
```

`--source` does two things:
- Sets `CERCLE_SOURCE=/path/to/project` in the environment for the `rlm` condition, so the agent's rlm scripts search the right index.
- Passes `?source=...` to the daemon's `/files` endpoint when fetching the file list for the `vanilla+files` condition.

If `--source` is omitted, the harness falls back to the `"source"` field in the task JSON, then to whatever `CERCLE_SOURCE` is already set in your shell environment.

---

## Quick start

```sh
# 1. Start the daemon
task run &

# 2. Index the target codebase
rlm/scripts/rlm-index /path/to/cercle

# 3. Run Claude under vanilla vs rlm — the key comparison
python eval/eval.py \
  --tasks eval/tasks/cercle.json \
  --source /path/to/cercle \
  --models claude \
  --conditions vanilla,rlm

# 4. Full run: both models, all three conditions
python eval/eval.py \
  --tasks eval/tasks/cercle.json \
  --source /path/to/cercle \
  --models claude,gemini \
  --conditions vanilla,vanilla+files,rlm
```

---

## Conditions

| Condition | What the agent gets |
|-----------|---------------------|
| `vanilla` | Nothing — bare question, no tools (`--tools ""` for Claude) |
| `vanilla+files` | The project file tree injected into the prompt, still no tools |
| `rlm` | Full tool access; the agent can call cercle's rlm scripts to search the codebase |

---

## All options

| Flag | Default | Description |
|------|---------|-------------|
| `--tasks` | *(required)* | Path to a task JSON file |
| `--source` | task file / env | Absolute path to the indexed codebase (sets `CERCLE_SOURCE`) |
| `--models` | `claude` | Comma-separated: `claude`, `gemini` |
| `--conditions` | `vanilla,rlm` | Comma-separated: `vanilla`, `vanilla+files`, `rlm` |
| `--addr` | `127.0.0.1:7770` | Daemon address (used for `/files` pre-fetch) |
| `--timeout` | `120` | Per-call timeout in seconds |
| `--out-dir` | `eval/results/` | Directory for result JSON files |

---

## Task file format

```json
{
  "description": "Q&A over the cercle codebase",
  "source": null,
  "tasks": [
    {
      "id": "q01",
      "question": "What SQL trigger invalidates stale embeddings when a document is re-indexed?",
      "expected_keywords": ["documents_embedding_invalidate"]
    }
  ]
}
```

- **`source`** — optional default codebase path; overridden by `--source` on the CLI.
- **`expected_keywords`** — all strings must appear (case-insensitive) in the agent's answer for the task to be scored `correct`. Use specific identifiers, function names, or constants that only appear in the target codebase — a vanilla model should not be able to guess them.

---

## Output

**Terminal** — a ✓/✗ table printed after all runs complete:

```
| id    | claude/vanilla | claude/rlm |
|-------|----------------|------------|
| q01   | ✗              | ✓          |
| q02   | ✗              | ✓          |
| score | 0/2            | 2/2        |
```

**JSON file** — saved to `eval/results/run-<timestamp>.json` with full details for every cell: answer text, latency, return code, and score (hits/missed keywords).

---

## Writing tasks for a new codebase

Good eval tasks have answers that:
- Are specific identifiers, constants, or names from the codebase (not general knowledge)
- Require at most one or two searches to find with rlm tools
- Are impossible to guess without reading the source

Bad tasks have answers that are generic (`"true"`, `"nil"`) or widely known (`"JSON"`, `"HTTP"`).
