# Session Compaction Summary

## User Intent
- Build an eval harness to measure RLM vs vanilla agent accuracy on codebase Q&A
- Verify the core cercle hypothesis: retrieval tools improve accuracy without proportionally increasing context use
- Keep everything runnable from the CLI with no API keys or SDK dependencies

## Contextual Work Summary

### Eval Harness (`eval/`)
- Created `eval/eval.py` with `--tasks` (batch) and `--question` (quick single-question) modes
- Conditions: `vanilla`, `vanilla+files`, `rlm`; models: `claude`, `gemini`
- Uses `claude -p --output-format stream-json --verbose` to capture token counts from stream events
- Sets `cwd=source` for rlm condition so rlm scripts' `$PWD` fallback resolves correctly
- `--tools ""` flag for vanilla is confirmed valid but was found not to disable tools in practice (active bug); vanilla was running from project cwd giving it accidental file access

### Task Files & Manual Eval Artifacts
- `eval/tasks/cercle.json`: 20 Q&A tasks, all verified against actual source
- `eval/questions.md` / `eval/answers.md`: human-readable question/answer pairs for manual prompting
- Manual eval run: RLM 20/20, Vanilla 19/20; RLM used 77% context vs 73% vanilla (4% overhead for one fewer mistake)
- Judge prompt designed to output a markdown comparison table with ✓/✗/~ scoring

### `rlm-index` Script
- New script `rlm/scripts/rlm-index` following the same pattern as other rlm scripts
- Indexes a directory via `POST /index`, then polls `POST /embed` every 1 second until `pending_embed` reaches 0
- Failures in the poll loop are non-fatal (`|| true`) to avoid `set -e` killing the script on transient errors
- Skips embedding wait if `semantic: false` (no vectors loaded)

### README Updates
- `eval/README.md`: rewrote to document `rlm-index` for indexing (not raw curl), `--source` semantics, all flags
- Main `README.md`: added Evaluation section with results table and link to `eval/README.md`, positioned after the motivation sections

## Files Touched

### Eval Harness
- **eval/eval.py**: full harness; stream-json parsing for token counts; `--question` flag; `cwd` per condition; error display inline
- **eval/tasks/cercle.json**: 20 verified Q&A tasks
- **eval/questions.md**: numbered questions for manual prompting
- **eval/answers.md**: ground truth answers with source locations
- **eval/README.md**: rewritten by user to document manual eval process, prompts, and results table

### RLM Scripts
- **rlm/scripts/rlm-index**: new script; indexes + waits for embedding with robust polling

### Documentation
- **README.md**: added Evaluation section with 19/20 vs 20/20 results table
