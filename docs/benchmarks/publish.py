#!/usr/bin/env python3
"""Publish only the two known synthetic runs; refuse unknown/private inputs."""
import argparse
import hashlib
import json
import pathlib
import subprocess

p = argparse.ArgumentParser(description=__doc__)
p.add_argument("clean_source", type=pathlib.Path)
p.add_argument("short_results", type=pathlib.Path)
p.add_argument("long_results", type=pathlib.Path)
args = p.parse_args()
source = args.clean_source.resolve()
revision = "58f9748dc370dfb80706cae0e13a08ca0acf35e3"

def git(*command):
    return subprocess.check_output(["git", "-C", str(source), *command], text=True).strip()

assert git("rev-parse", "HEAD") == revision
assert not git("status", "--porcelain"), "source clone must be clean"
files = git("ls-files", "*.go", "go.mod", "bench/go/go.mod", "bench/go/go.sum", "bench/rust/Cargo.toml", "bench/rust/Cargo.lock", "bench/rust/src/*.rs", "bench/run.py").splitlines()
report = {
    "sourceCommit": revision,
    "sourceFilesSha256": {name: hashlib.sha256((source / name).read_bytes()).hexdigest() for name in files},
    "provenance": "Workers rebuilt from a standalone clean clone, not a linked worktree. Rust built --release --locked from the same clone. Go embedded VCS revision matches sourceCommit with modified=false.",
    "scope": "Synthetic inputs only. Native streaming I/O plus parser, not preprocessing or service memory/concurrency safety. Self-verification.",
    "datasets": {},
}
expected = {
    "short": "a67f7f796b9eca74180dc5c9bd928b2ace690546234d113f2d231ae100e9ef6b",
    "long": "466b004a2fec9ee2b00adc5de9f11b1df6afe7ed9b7e4be550cc0dadd306caf3",
}
for name, path in [("short", args.short_results), ("long", args.long_results)]:
    metadata = json.loads((path / "metadata.json").read_text())
    assert metadata["inputSha256"] == expected[name], "refusing an unknown corpus"
    assert metadata["sourceHead"] == revision
    assert "vcs.revision=" + revision in metadata["goBuild"]
    assert "vcs.modified=false" in metadata["goBuild"]
    # Remove the machine-local executable pathname, not any build/version evidence.
    metadata["goBuild"] = "go-bench: " + metadata["goBuild"].split(": ", 1)[1]
    results = json.loads((path / "results.json").read_text())
    verification = json.loads((path / "verification.json").read_text())
    assert len(results) == 15 and all(r["rows"] == 80000 and r["unmatched"] == 0 for r in results)
    assert all(not v["differentFields"] for v in verification.values())
    report["datasets"][name] = {"metadata": metadata, "verification": verification, "runs": results, "summary": json.loads((path / "summary.json").read_text())}
pathlib.Path(__file__).with_name("results.json").write_text(json.dumps(report, indent=2) + "\n")
