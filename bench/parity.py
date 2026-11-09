#!/usr/bin/env python3
"""Compare synthetic training and parsing traces against the pinned Rust port."""
import argparse
import itertools
import json
import pathlib
import random
import subprocess

PIN = "9d5e8e8cad620bc2b83064e8c67e29affccf571c"


def cases():
    rng = random.Random(27846)
    lines = ["", " \t\x1c", "single", "single", "other", "<*> a", "<*> b"]
    for i in range(73):
        prefix = rng.choice(["auth", "worker", "net", "α", "7", "host2", "<*>"])
        middle = rng.choice(["opened", "closed", "failed", "retry", "<*>"])
        line = f"{prefix} {middle} user={rng.randrange(8)} route::{rng.randrange(5)} {rng.choice(['ok', 'error', 'busy'])}"
        lines.append(line.replace(" ", rng.choice([" ", "\t", "\x1f", "\u00a0"])))
    queries = lines[::7] + ["new prefix totally unmatched", "auth opened user=99 route::9 error", "single", "<*>"]
    for depth, threshold, children, numeric in itertools.product([3, 4, 5, 7], [0, .3, .4, .75, 1], [1, 2, 100], [False, True]):
        cfg = {"depth": depth, "sim_threshold": threshold, "max_children": children, "parametrize_numeric_tokens": numeric}
        yield {"name": f"tree-d{depth}-s{threshold}-c{children}-n{int(numeric)}", "request": {"config": cfg, "lines": lines, "queries": queries}}
    for extra in [[], ["="], ["=", "::"], ["a", "ab"], ["ab", "a"], ["_", "é"], ["", "."]]:
        yield {"name": f"delimiters-{extra}", "request": {"config": {"extra_delimiters": extra}, "lines": lines + ["abc é_x.y", "k=v1, x=1", "k=v2, x=2"], "queries": queries + ["k=abc, x=99", "abc é_x.y"]}}
    rules = [
        [{"pattern": "[0-9]+", "mask": "<N>"}],
        [{"pattern": "(user=)([0-9]+)", "mask": "${1}<ID>"}],
        [{"pattern": "[0-9]+", "mask": "<N>"}, {"pattern": "<N>:<N>", "mask": "<PAIR>"}],
        [{"pattern": "(é)([0-9]+)", "mask": "${1}<N>"}],
        [{"pattern": "(?P<key>user=)([0-9]+)", "mask": "${key}<ID>"}],
        [{"pattern": "[0-9]+", "mask": ""}],
        [{"pattern": "a*", "mask": "X"}],
        [{"pattern": "", "mask": "-"}],
        [{"pattern": "a", "mask": "$$"}],
        [{"pattern": "[0-9]+", "mask": "<*>"}],
        [{"pattern": "\\p{N}+", "mask": "<N>"}],
    ]
    mask_lines = ["worker thread-0 code=-1 from 10.0.0.1 took 42.5ms on user_0", "worker thread-3 code=-7 from 10.0.0.2 took 13.0ms on user_0", "pair 12:34 item 5ms stage a", "pair 56:78 item 7s stage b", "x é123 y", "x é456 y", "unicode Ⅳ ٢ ²", "aba", "", "user=123", "user=456"]
    for i, masking in enumerate(rules):
        yield {"name": f"mask-{i}", "request": {"config": {"masking": masking}, "lines": mask_lines, "queries": mask_lines + ["pair 90:12 item 6h stage c", "x é789 y", "user=789"]}}
    for length, threshold in [(10, .3), (10, .7), (59, .7), (100, .29), (7, 3 / 7)]:
        same = int(length * threshold)
        a = " ".join(f"a{i}" for i in range(length))
        b = " ".join(f"a{i}" if i < same else f"b{i}" for i in range(length))
        yield {"name": f"threshold-{length}-{threshold}", "request": {"config": {"depth": 3, "sim_threshold": threshold}, "lines": [a, b, a, b], "queries": [a, b]}}


def invoke(binary, requests):
    raw = "".join(json.dumps(r, ensure_ascii=False) + "\n" for r in requests)
    result = subprocess.run([str(binary)], input=raw, text=True, capture_output=True, check=True)
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    if len(rows) != len(requests):
        raise RuntimeError(f"expected {len(requests)} responses, received {len(rows)}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--rust", type=pathlib.Path, required=True)
    p.add_argument("--go", type=pathlib.Path, required=True)
    p.add_argument("--write-golden", type=pathlib.Path)
    args = p.parse_args()
    tests = list(cases())
    requests = [t["request"] for t in tests]
    rust = invoke(args.rust, requests)
    go = invoke(args.go, requests)
    failures = []
    for case, expected, actual in zip(tests, rust, go):
        if expected != actual:
            failures.append(case["name"])
            print(json.dumps({"case": case["name"], "request": case["request"], "rust": expected, "go": actual}, ensure_ascii=False))
    if failures:
        raise SystemExit(f"{len(failures)}/{len(tests)} parity cases failed")
    if args.write_golden:
        args.write_golden.parent.mkdir(parents=True, exist_ok=True)
        with args.write_golden.open("w") as f:
            for case, expected in zip(tests, rust):
                f.write(json.dumps({"upstream": PIN, **case, "expected": expected}, ensure_ascii=False, separators=(",", ":")) + "\n")
    print(json.dumps({"upstream": PIN, "cases": len(tests), "training_steps": sum(len(r["lines"]) for r in requests), "queries": sum(len(r["queries"]) for r in requests), "search_strategies": 3, "status": "pass"}))


if __name__ == "__main__":
    main()
