# Synthetic results and provenance

The [README table](../../README.md#measured-results) comes from new clean-clone runs, **not** the earlier worktree runs with an ambiguous embedded VCS stamp. Measurements are descriptive, not a general speedup claim.

## What was measured

- 80,000 records per dataset, three partitions, sorted by `(partition, message)` before measurement. Short lines come from `bench/run.py`; [make-long.py](make-long.py) generates invented 2,048-byte ASCII lines. **No customer input or private templates.**
- Each worker streams the same framed file twice: ordered training, then exact tree-only frozen matching. No full-input retention inside the worker.
- Depth 4, similarity 0.4, 100 children, unlimited clusters, numeric routing **off**, no masks. These are comparison settings, not a claim that all library defaults are identical.
- One full verification/warmup per parser, then five measured rounds in seeded randomized parser order. Go workers use `GOMAXPROCS=2`; no CPU affinity or memory cap. Native macOS/arm64, Go 1.25.11, rustc 1.97.1. Both datasets have zero unmatched messages and identical full training/frozen assignments, templates, IDs, and counts across all three implementations.
- Training/matching timers include buffered file I/O and framing. Peak RSS comes from `/usr/bin/time -l` and includes runtime, buffers, model, and result serialization. Sorting/preprocessing/caller input retention are excluded. **Not a service, concurrency, accuracy, or 300Mi memory-safety test.**

Raw nanosecond timings, RSS bytes, median/min/max summaries, deterministic digests, settings, compiler metadata, source-file hashes, and executable hashes are in [results.json](results.json). README values round medians to two decimals: ns / 1,000,000 and RSS bytes / 1,048,576. No outlier removal. The long-input Go port is slower than Kloudmate in this run; lower Rust component RSS does not establish end-to-end safety.

## Source and executable identity

Both workers were rebuilt from a **standalone clean clone** at [`58f9748dc370dfb80706cae0e13a08ca0acf35e3`](https://github.com/sematext/drain3-go/tree/58f9748dc370dfb80706cae0e13a08ca0acf35e3). The Go executable reports that exact VCS revision and `vcs.modified=false`. Rust was freshly built from the same clone with `--release --locked`.

- Rust library: `9d5e8e8cad620bc2b83064e8c67e29affccf571c` (Apache-2.0).
- Kloudmate: `bc42b50af108a7a7ce926a1534d78c9cf9373eed` (MIT).
- Corpus SHA-256: short `a67f7f796b9eca74180dc5c9bd928b2ace690546234d113f2d231ae100e9ef6b`; long `466b004a2fec9ee2b00adc5de9f11b1df6afe7ed9b7e4be550cc0dadd306caf3`.

These hashes identify the measured artifacts; cross-machine reproducible binary builds are not claimed. Full per-message verification traces were checked before timing; public results retain their digests and comparison outcomes, not large trace arrays.

## Reproduce

Build from a clean clone, using a temporary output directory **outside** it. The pinned commit contains the benchmark workers and runner; use the long generator from the current docs checkout:

```sh
DOCS=$PWD
D=$(mktemp -d)
git clone https://github.com/sematext/drain3-go.git "$D/source"
cd "$D/source"
git checkout --detach 58f9748dc370dfb80706cae0e13a08ca0acf35e3
(cd bench/go && GOTOOLCHAIN=go1.25.11 go build -o "$D/go-bench" .)
cargo build --release --locked --manifest-path bench/rust/Cargo.toml \
  --target-dir "$D/rust-target"
go version -m "$D/go-bench" # check VCS revision and modified=false
python3 bench/run.py --go "$D/go-bench" \
  --rust "$D/rust-target/release/drain3-go-reference" --out "$D/short" --runs 5
python3 "$DOCS/docs/benchmarks/make-long.py" "$D/long.frames"
python3 bench/run.py --go "$D/go-bench" \
  --rust "$D/rust-target/release/drain3-go-reference" \
  --input "$D/long.frames" --out "$D/long" --runs 5
```

Use rustc 1.97.1 to match this compiler version. Run `publish.py "$D/source" "$D/short" "$D/long"` from the docs checkout to regenerate the public JSON. It refuses unknown corpus hashes, dirty sources, and mismatched embedded Go provenance.

For the separate 143-case golden comparison, see [bench/README.md](../../bench/README.md). That check is self-verification, not universal parity or independent review.
