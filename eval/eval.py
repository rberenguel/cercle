#!/usr/bin/env python3
"""cercle eval harness — measures RLM vs vanilla agent accuracy on codebase Q&A."""

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.request
import urllib.parse
from datetime import datetime, timezone


# ---------------------------------------------------------------------------
# CLI command builders
# ---------------------------------------------------------------------------

def build_cmd(cli: str, condition: str, question: str, filelist: str | None = None) -> list[str]:
    if cli == "claude":
        base = ["claude", "-p", "--output-format", "stream-json", "--verbose"]
        if condition == "vanilla":
            # --tools "" strips tool access; re-enable once RLM skill loading is confirmed working
            return base + ["--tools", "", question]
        elif condition == "vanilla+files":
            prompt = f"The project contains these files:\n{filelist}\n\nQuestion: {question}"
            return base + ["--tools", "", prompt]
        else:  # rlm
            return base + [question]

    elif cli == "gemini":
        if condition == "vanilla":
            return ["gemini", "-y", "--no-tools", question]
        elif condition == "vanilla+files":
            prompt = f"The project contains these files:\n{filelist}\n\nQuestion: {question}"
            return ["gemini", "-y", "--no-tools", prompt]
        else:  # rlm
            return ["gemini", "-y", question]

    raise ValueError(f"Unknown CLI: {cli}")


# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------

def run_condition(
    cli: str,
    condition: str,
    question: str,
    filelist: str | None,
    timeout: int,
    env: dict | None = None,
    cwd: str | None = None,
) -> dict:
    cmd = build_cmd(cli, condition, question, filelist)
    t0 = time.monotonic()
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
            env=env,
            cwd=cwd,
        )
        elapsed = time.monotonic() - t0
        raw = result.stdout.strip()
        answer = raw
        input_tokens = None
        output_tokens = None
        if cli == "claude":
            text_parts = []
            for line in raw.splitlines():
                line = line.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line)
                    # Unwrap Claude Code's stream_event envelope if present
                    event = obj.get("event", obj) if obj.get("type") == "stream_event" else obj
                    etype = event.get("type", "")
                    if etype == "content_block_delta":
                        delta = event.get("delta", {})
                        if delta.get("type") == "text_delta":
                            text_parts.append(delta.get("text", ""))
                    elif etype == "message_start":
                        usage = event.get("message", {}).get("usage", {})
                        input_tokens = usage.get("input_tokens")
                    elif etype == "message_delta":
                        usage = event.get("usage", {})
                        output_tokens = usage.get("output_tokens")
                        if usage.get("input_tokens"):
                            input_tokens = usage.get("input_tokens")
                    elif etype == "result":
                        # Fallback: plain json output format
                        answer = event.get("result", answer)
                except json.JSONDecodeError:
                    continue
            if text_parts:
                answer = "".join(text_parts)
        return {
            "answer": answer,
            "error": result.stderr.strip() if result.returncode != 0 else None,
            "returncode": result.returncode,
            "latency_s": round(elapsed, 2),
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
        }
    except subprocess.TimeoutExpired:
        return {
            "answer": "",
            "error": f"timeout after {timeout}s",
            "returncode": -1,
            "latency_s": round(time.monotonic() - t0, 2),
        }
    except FileNotFoundError:
        return {
            "answer": "",
            "error": f"CLI not found: {cli!r}",
            "returncode": -1,
            "latency_s": 0.0,
        }


# ---------------------------------------------------------------------------
# Scoring
# ---------------------------------------------------------------------------

def score(answer: str, task: dict) -> dict:
    keywords = task.get("expected_keywords", [])
    hits = [k for k in keywords if k.lower() in answer.lower()]
    missed = [k for k in keywords if k not in hits]
    return {
        "correct": len(hits) == len(keywords),
        "hits": hits,
        "missed": missed,
    }


# ---------------------------------------------------------------------------
# File list pre-fetch
# ---------------------------------------------------------------------------

def fetch_filelist(addr: str, source: str | None) -> str:
    params = {}
    if source:
        params["source"] = source
    url = f"http://{addr}/files"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    try:
        with urllib.request.urlopen(url, timeout=10) as resp:
            data = json.loads(resp.read())
        return "\n".join(data.get("files", []))
    except Exception as exc:
        print(f"[warn] could not fetch file list from {url}: {exc}", file=sys.stderr)
        return ""


# ---------------------------------------------------------------------------
# Table rendering
# ---------------------------------------------------------------------------

TICK = "\u2713"
CROSS = "\u2717"


def render_table(tasks: list[dict], results: dict, models: list[str], conditions: list[str]) -> str:
    cols = [f"{m}/{c}" for m in models for c in conditions]
    col_w = max(len(c) for c in cols) + 1
    id_w = max(len(t["id"]) for t in tasks)

    def row(id_cell: str, cells: list[str]) -> str:
        return "| " + f"{id_cell:<{id_w}}" + " | " + " | ".join(f"{c:<{col_w}}" for c in cells) + " |"

    sep = "|" + "-" * (id_w + 2) + "|" + "|".join("-" * (col_w + 2) for _ in cols) + "|"

    lines = [row("id", cols), sep]

    totals = {c: 0 for c in cols}
    for task in tasks:
        cells = []
        for m in models:
            for c in conditions:
                res = results.get((task["id"], m, c), {})
                correct = res.get("score", {}).get("correct", False)
                cells.append(TICK if correct else CROSS)
                if correct:
                    totals[f"{m}/{c}"] += 1
        lines.append(row(task["id"], cells))

    lines.append(sep)
    n = len(tasks)
    score_cells = [f"{totals[f'{m}/{c}']}/{n}" for m in models for c in conditions]
    lines.append(row("score", score_cells))

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="cercle eval harness")
    parser.add_argument("--tasks", default=None, help="Path to task JSON file")
    parser.add_argument("--question", default=None, help="Single question to run (no task file needed)")
    parser.add_argument("--models", default="claude", help="Comma-separated: claude,gemini")
    parser.add_argument("--conditions", default="vanilla,rlm",
                        help="Comma-separated: vanilla,vanilla+files,rlm")
    parser.add_argument("--addr", default="127.0.0.1:7770", help="Daemon address")
    parser.add_argument("--source", default=None, help="Source namespace (CERCLE_SOURCE)")
    parser.add_argument("--timeout", type=int, default=120, help="Per-call timeout in seconds")
    parser.add_argument("--out-dir", default=os.path.join(os.path.dirname(__file__), "results"),
                        help="Directory for result JSON files")
    args = parser.parse_args()

    if not args.tasks and not args.question:
        parser.error("one of --tasks or --question is required")

    models = [m.strip() for m in args.models.split(",")]
    conditions = [c.strip() for c in args.conditions.split(",")]

    if args.question:
        tasks = [{"id": "q", "question": args.question, "expected_keywords": []}]
        source = args.source
    else:
        with open(args.tasks) as f:
            task_data = json.load(f)
        tasks = task_data["tasks"]
        source = args.source or task_data.get("source")

    print(f"Loaded {len(tasks)} tasks | models={models} | conditions={conditions}")

    # Pre-fetch file list if needed
    filelist = None
    if "vanilla+files" in conditions:
        print(f"Fetching file list from {args.addr}…")
        filelist = fetch_filelist(args.addr, source)
        print(f"  Got {len(filelist.splitlines())} files")

    # Build environment for rlm condition (needs CERCLE_SOURCE)
    base_env = os.environ.copy()
    rlm_env = base_env.copy()
    if source:
        rlm_env["CERCLE_SOURCE"] = source

    results: dict = {}
    total = len(tasks) * len(models) * len(conditions)
    done = 0

    for task in tasks:
        for model in models:
            for condition in conditions:
                done += 1
                print(f"[{done}/{total}] {task['id']} | {model}/{condition} … ", end="", flush=True)

                env = rlm_env if condition == "rlm" else base_env
                # Run rlm condition from the source directory so $PWD fallback works in rlm scripts
                cwd = source if (condition == "rlm" and source and os.path.isdir(source)) else None
                res = run_condition(model, condition, task["question"], filelist, args.timeout, env, cwd)

                s = score(res["answer"], task)
                res["score"] = s
                results[(task["id"], model, condition)] = res

                tok = res.get("input_tokens")
                tok_str = f"  [{tok} tok]" if tok is not None else ""
                if s["correct"] is True:
                    sym = TICK
                    suffix = tok_str
                elif not task["expected_keywords"]:
                    sym = "-"
                    suffix = tok_str
                else:
                    sym = CROSS
                    suffix = f"  missed={s['missed']}{tok_str}"
                if res.get("error"):
                    suffix += f"  ERR: {res['error'][:80]}"
                print(f"{sym}{suffix}")

    print()
    if args.question:
        # Single-question mode: print answers and token usage side by side
        for model in models:
            for condition in conditions:
                res = results.get(("q", model, condition), {})
                tok_in = res.get("input_tokens", "?")
                tok_out = res.get("output_tokens", "?")
                answer = res.get("answer", "")
                print(f"=== {model}/{condition}  [in={tok_in} out={tok_out}] ===")
                print(answer[:2000] + ("…" if len(answer) > 2000 else ""))
                print()
    else:
        print(render_table(tasks, results, models, conditions))
        print()

    # Save results
    os.makedirs(args.out_dir, exist_ok=True)
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    out_path = os.path.join(args.out_dir, f"run-{ts}.json")

    serializable = {}
    for (task_id, model, condition), res in results.items():
        serializable.setdefault(task_id, {})[f"{model}/{condition}"] = res

    with open(out_path, "w") as f:
        json.dump({
            "timestamp": ts,
            "models": models,
            "conditions": conditions,
            "source": source,
            "tasks_file": args.tasks,
            "results": serializable,
        }, f, indent=2)
    print(f"Results saved to {out_path}")


if __name__ == "__main__":
    main()
