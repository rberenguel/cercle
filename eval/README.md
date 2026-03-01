# Evaluating this

Ideally I would run this automatically, but I can't get several pieces to work (Claude reporting usage properly, Claude using the RLM skill, for some reason it is not shown as one it can use, but I could invoke it…). Also, **this should be a way larger codebase for it to be realistic**. This is just an eyeballing on whether this improves anything.

```
task reset
task index DIR=/Users/ruben/code/cercle # Or wherever this got
```

The following was run in blank Claude Code CLI sessions. They had access to the same memories otherwise.

## RLM prompt

> I want you to read @~/.claude/skills/rlm/SKILL.md and use the tooling provided by the skill to answer the questions in @eval/cercle/questions.md Do not read ANY other files directly, this is a test of the tooling. You do not need to run the summary task, but write your answers to eval/cercle/rlm-answers.md as bulleted answers (same format as the questions)

This left the context after writing the answers at **77% usage** (Sonnet 4.6 at 1st March, 2026)

### Harder bonus question

> I want you to read @~/.claude/skills/rlm/SKILL.md and use the tooling provided by the skill to answer "Walk through the full sequence of events that causes a chunk document to be re-embedded after its source file is modified on disk. Name each component involved." Do not read ANY files directly, this is a test of the tooling. You do not need to run the summary task

From cold (indexed and embedded, fresh session), 79% context available after an answer. The cercle codebase is likely too small to be worth the effort of not "just read the whole damn thing"?

## Vanilla prompt

> I want you to answer the questions in @eval/cercle/questions.md and write your answers to eval/cercle/vanilla-answers.md as bulleted answers (same format as the questions)

This left the context after writing the answers at **74% usage** (Sonnet 4.6 at 1st March, 2026)

### Harder bonus question

> Answer this: "Walk through the full sequence of events that causes a chunk document to be re-embedded after its source file is modified on disk. Name each component involved."

From cold (indexed and embedded, fresh session), 83% context available after an answer. The `cercle` codebase is likely too small to be worth the effort of not "just read the whole damn thing"?

## Judge prompt (for the 20 retrieval questions)

> Read @eval/cercle/answers.md (ground truth), @eval/cercle/vanilla-answers.md, and @eval/cercle/rlm-answers.md.
> 
> For each question 1–20, judge whether each answer is correct based on the ground truth. An answer is correct if
> it contains the key fact — exact identifier, value, or name — regardless of surrounding explanation. It is wrong
> if it omits, misnames, or halluccinates the key fact.
> 
> Output a single markdown table:
> 
> | # | Key fact | Vanilla | RLM | Notes |
> |---|----------|---------|-----|-------|
> 
> - **Key fact**: the specific identifier or value from answers.md (keep it short)
> - **Vanilla / RLM**: ✓ correct, ✗ wrong, or ~ partial (right idea, missing precision)
> - **Notes**: only if there is something interesting to say (e.g. one answer has extra correct detail, one
>   halluccinates)
> 
> End with a score row: `| **Score** | | X/20 | X/20 | |`
> 
> The ~ partial is important — you'll probably see cases where one answer gets the concept right but uses a
> slightly wrong name or misses a specific value.

### Judgement

| # | Key fact | Vanilla | RLM | Notes |
 |---|----------|---------|-----|-------|
 | 1 | `documents_embedding_invalidate` | ✓ | ✓ | |
 | 2 | `''` (empty string for both markers) | ✓ | ✓ | |
 | 3 | `100` | ✓ | ✓ | |
 | 4 | `7770` | ✓ | ✓ | |
 | 5 | `-reset` | ✓ | ✓ | |
 | 6 | `parent_id` | ✓ | ✓ | |
 | 7 | `glove-local-dim100` | ✓ | ✓ | |
 | 8 | `CERCLE_SOURCE` | ✓ | ✓ | |
 | 9 | `::` and `@` | ✓ | ✓ | |
 | 10 | `0.5` | ✓ | ✓ | |
 | 11 | `node_modules` and `vendor` | ~ | ✓ | Vanilla leads with `.git` + `node_modules`, omitting `vendor` from
 the headline pair |
 | 12 | `DeleteSummary` | ✓ | ✓ | |
 | 13 | `50` | ✓ | ✓ | |
 | 14 | `codesign` | ~ | ✓ | Vanilla gives the full command but ground truth key fact is `codesign`; RLM matches
 the fuller canonical form |
 | 15 | `sqlite_fts5` | ✓ | ✓ | |
 | 16 | `4` | ✓ | ✓ | |
 | 17 | `glove.6B` | ✓ | ✓ | |
 | 18 | `start_line` and `end_line` | ✓ | ✓ | |
 | 19 | `github.com/mattn/go-sqlite3` | ✓ | ✓ | |
 | 20 | Embedder is nil, semantic search disabled | ✓ | ✓ | |
 | **Score** | | 19.5/20 | 20/20 | |

## Bonus question analysis

I verified this with Gemini 3.1 Pro and Claude as isolated judges against the codebase with each explanation.

It would look like vanilla wins on this small codebase, costing 4% less context for this relatively complex question. But, actually, the explanation it gives is wrong: the RLM based answer _is_ correct, with both Gemini and Claude agreeing it is quite a lot better in terms of precision and accuracy.