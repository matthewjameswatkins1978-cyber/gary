#!/usr/bin/env python3
"""Temporary fail-closed runtime guard for GARY paid worker entry points.

Delete this helper once the same protections live permanently in Go source.
"""

from pathlib import Path
import argparse


def require_replace(text: str, old: str, new: str, label: str, count: int = 0) -> str:
    if old not in text:
        raise SystemExit(f"cost guard anchor missing: {label}")
    if count <= 0:
        return text.replace(old, new)
    return text.replace(old, new, count)


def patch_run() -> None:
    path = Path("cmd/gary/run.go")
    src = path.read_text()

    src = require_replace(
        src,
        "maxAttempts                = 3",
        "maxAttempts                = 2",
        "maxAttempts",
        1,
    )
    src = require_replace(
        src,
        "maxAttempts, true, defaultWorkerMaxRunSec",
        "maxAttempts, false, defaultWorkerMaxRunSec",
        "automatic closer",
    )
    src = require_replace(
        src,
        'if attempt > 1 && lastPatch != "" {',
        'if attempt > 1 && lastFailure != "" {',
        "retry evidence",
        1,
    )

    loop = "\tfor attempt := 1; attempt <= attemptLimit; attempt++ {\n"
    guarded_loop = (
        loop
        + "\t\tif cfg.MaxCostUSD > 0 {\n"
        + "\t\t\testimated := EstimateJobCostForModel(model, attempt)\n"
        + "\t\t\tallowed, reason := BudgetCheck(cfg.MaxCostUSD, 0, estimated)\n"
        + "\t\t\tif !allowed {\n"
        + "\t\t\t\tlogPrintln(prefix, \"  BUDGET_STOPPED: \"+reason)\n"
        + "\t\t\t\tfinalStatus = \"BUDGET_STOPPED\"\n"
        + "\t\t\t\tbreak\n"
        + "\t\t\t}\n"
        + "\t\t}\n"
    )
    src = require_replace(src, loop, guarded_loop, "attempt budget check", 1)

    start_marker = '\t\tlogPrintln(prefix, "  GENERATING REPAIR BRIEF")\n'
    end_marker = "\t\tlastFailure = formatRepairBrief(brief)\n"
    start = src.find(start_marker)
    end = src.find(end_marker, start)
    if start < 0 or end < 0:
        raise SystemExit("cost guard anchor missing: paid repair brief")
    end += len(end_marker)
    src = src[:start] + '\t\tlogPrintln(prefix, "  RETRYING WITH FAILURE EVIDENCE")\n' + src[end:]

    required = [
        "maxAttempts                = 2",
        "maxAttempts, false, defaultWorkerMaxRunSec",
        'if attempt > 1 && lastFailure != ""',
        "RETRYING WITH FAILURE EVIDENCE",
        "EstimateJobCostForModel(model, attempt)",
    ]
    for marker in required:
        if marker not in src:
            raise SystemExit(f"cost guard verification failed: {marker}")
    if "GENERATING REPAIR BRIEF" in src:
        raise SystemExit("paid repair-brief path still active")

    path.write_text(src)


def patch_master() -> None:
    path = Path("cmd/gary/master.go")
    src = path.read_text()
    old = (
        "\tif m.MaxWorkers == 0 {\n"
        "\t\tm.MaxWorkers = masterWorkerLimit()\n"
        "\t}\n"
        "\tif m.MaxWorkers < 1 || m.MaxWorkers > 8 {\n"
    )
    new = (
        "\tworkerLimit := masterWorkerLimit()\n"
        "\tif m.MaxWorkers == 0 || m.MaxWorkers > workerLimit {\n"
        "\t\tm.MaxWorkers = workerLimit\n"
        "\t}\n"
        "\tif m.MaxWorkers < 1 || m.MaxWorkers > 8 {\n"
    )
    src = require_replace(src, old, new, "master worker ceiling", 1)
    if "m.MaxWorkers > workerLimit" not in src:
        raise SystemExit("master worker ceiling verification failed")
    path.write_text(src)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--master", action="store_true", help="also apply the master worker ceiling")
    args = parser.parse_args()
    patch_run()
    if args.master:
        patch_master()


if __name__ == "__main__":
    main()
