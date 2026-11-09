#!/usr/bin/env python3
"""Run isolated streaming comparisons; retain raw results outside the repository."""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import random
import statistics
import struct
import subprocess
import time

ROOT = pathlib.Path(__file__).resolve().parents[1]


def synthetic(path, rows):
    records = []
    for i in range(rows):
        accepted = i % 7 != 0
        line = f"component-{i % 31} {'accepted' if accepted else 'rejected'} user-{i % 997} item-{i % 127} status {'ok' if accepted else 'error'}"
        records.append((i % 3, line.encode()))
    records.sort()
    with path.open("wb") as f:
        for partition, line in records:
            f.write(struct.pack("<II", partition, len(line)))
            f.write(line)


def sha(path):
    h = hashlib.sha256()
    with path.open("rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def command(args):
    return subprocess.check_output(args, text=True, stderr=subprocess.STDOUT).strip()


def execute(name, cmd, output, rss_file, trace=False):
    actual = cmd + (["--trace"] if trace else [])
    if platform.system() == "Darwin":
        timed = ["/usr/bin/time", "-l", "-o", str(rss_file)] + actual
    elif platform.system() == "Linux":
        timed = ["/usr/bin/time", "-v", "-o", str(rss_file)] + actual
    else:
        raise RuntimeError("RSS collection supports Darwin and Linux only")
    start = time.monotonic_ns()
    with output.open("w") as f:
        subprocess.run(timed, stdout=f, check=True, env={**os.environ, "GOMAXPROCS": "2"}, timeout=180)
    wall = time.monotonic_ns() - start
    rss = None
    for line in rss_file.read_text().splitlines():
        if "maximum resident set size" in line and platform.system() == "Darwin":
            rss = int(line.split()[0])
        elif "Maximum resident set size (kbytes):" in line:
            rss = int(line.split(":", 1)[1]) * 1024
    if rss is None:
        raise RuntimeError("peak RSS was not captured")
    r = json.loads(output.read_text())
    return {"parser": name, "wallNs": wall, "rssBytes": rss, **{k: r[k] for k in ["rows", "trainNs", "matchNs", "trainDigest", "matchDigest", "unmatched"]}, "clusterCount": len(r["clusters"])}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", type=pathlib.Path, required=True, help="Go comparison binary")
    p.add_argument("--rust", type=pathlib.Path, required=True, help="Rust reference binary")
    p.add_argument("--input", type=pathlib.Path, help="existing binary partition/message frames")
    p.add_argument("--synthetic-rows", type=int, default=80000)
    p.add_argument("--out", type=pathlib.Path, required=True)
    p.add_argument("--runs", type=int, default=5)
    p.add_argument("--numeric", action="store_true")
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        raise SystemExit("write benchmark artifacts outside the source repository")
    if args.runs < 3 or args.synthetic_rows < 1:
        raise SystemExit("at least three runs and one synthetic row are required")
    if out.exists() and any(out.iterdir()):
        raise SystemExit("output directory must be empty")
    out.mkdir(mode=0o700, parents=True, exist_ok=True)
    source = args.input.resolve() if args.input else out / "synthetic.frames"
    if not args.input:
        synthetic(source, args.synthetic_rows)
    commands = {
        "go": [str(args.go.resolve()), "--input", str(source), "--parser", "go"],
        "kloudmate": [str(args.go.resolve()), "--input", str(source), "--parser", "kloudmate"],
        "rust": [str(args.rust.resolve()), "bench", "--input", str(source)],
    }
    if args.numeric:
        for c in commands.values():
            c.append("--numeric")
    metadata = {"platform": platform.platform(), "machine": platform.machine(), "goBuild": command(["go", "version", "-m", str(args.go.resolve())]), "rustc": command(["rustc", "--version"]), "sourceHead": command(["git", "-C", str(ROOT), "rev-parse", "HEAD"]), "inputSha256": sha(source), "inputBytes": source.stat().st_size, "goBinarySha256": sha(args.go), "rustBinarySha256": sha(args.rust), "settings": {"depth": 4, "similarityThreshold": .4, "maxChildren": 100, "maxClusters": 0, "parametrizeNumericTokens": args.numeric, "masks": [], "matchStrategy": "never", "goMaxProcs": 2}, "runs": args.runs, "scope": "native streaming I/O plus parser; no retained preprocessing buffers or process memory cap"}
    (out / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    verification = {}
    for name, cmd in commands.items():
        output = out / f"verify-{name}.json"
        execute(name, cmd, output, out / f"verify-{name}.rss", trace=True)
        r = json.loads(output.read_text())
        verification[name] = {k: r[k] for k in ["rows", "trainDigest", "matchDigest", "unmatched", "clusters", "trainIds", "matchIds"]}
    baseline = verification["rust"]
    comparisons = {}
    for name, actual in verification.items():
        differing = [k for k in baseline if baseline[k] != actual[k]]
        comparisons[name] = {
            "differentFields": differing,
            "trainingAssignmentDifferences": sum(a != b for a, b in zip(baseline["trainIds"], actual["trainIds"])),
            "frozenAssignmentDifferences": sum(a != b for a, b in zip(baseline["matchIds"], actual["matchIds"])),
        }
        if name == "go" and differing:
            raise SystemExit(f"Go semantic mismatch against Rust: {differing}; raw evidence retained privately")
    (out / "verification.json").write_text(json.dumps(comparisons, indent=2) + "\n")
    results = []
    rng = random.Random(27846)
    for run in range(args.runs):
        names = list(commands)
        rng.shuffle(names)
        for name in names:
            r = execute(name, commands[name], out / f"run-{run}-{name}.json", out / f"run-{run}-{name}.rss")
            for key in ["rows", "trainDigest", "matchDigest", "unmatched"]:
                if r[key] != verification[name][key]:
                    raise SystemExit(f"nondeterministic {key} in {name}; raw evidence retained privately")
            r["run"] = run
            results.append(r)
            print(json.dumps(r), flush=True)
    (out / "results.json").write_text(json.dumps(results, indent=2) + "\n")
    summary = {}
    for name in commands:
        rows = [r for r in results if r["parser"] == name]
        summary[name] = {key: {"median": statistics.median(r[key] for r in rows), "min": min(r[key] for r in rows), "max": max(r[key] for r in rows)} for key in ["trainNs", "matchNs", "wallNs", "rssBytes"]}
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps({"goRustParity": "pass", "comparisons": comparisons, "rows": baseline["rows"], "learnedClusters": len(baseline["clusters"]), "unmatched": baseline["unmatched"], "summary": summary}))


if __name__ == "__main__":
    main()
