# Source provenance

## Implementation

The Go library is a port of [akshatagarwl/drain3 at `9d5e8e8cad620bc2b83064e8c67e29affccf571c`](https://github.com/akshatagarwl/drain3/tree/9d5e8e8cad620bc2b83064e8c67e29affccf571c). Its `LICENSE` states Apache-2.0 and Copyright 2026 Akshat Agarwal. The original notice is retained in `third_party/drain3-LICENSE.txt`, alongside the full Apache-2.0 license at the repository root.

The port follows `src/config.rs`, `src/drain.rs`, `src/mask.rs`, and `src/tokenize.rs`. It replaces Rust-specific storage and APIs with Go-owned strings, a mutex-protected miner, and a permanently frozen matcher. Modified source files identify the port and its license. [The semantic map](docs/semantics.md) describes the preserved rules and changed resource policy.

No other Go Drain implementation is an implementation source. Benchmark dependencies are separate from the dependency-free Go library.

## Earlier attribution

The Rust rewrite at `ca45715` describes a from-scratch, logpai-compatible implementation. Its tests mark cases as ported from logpai/Drain3. That project's [MIT license at `0fb6d6e1828838fa05c8f6fa5cda878e02ea79c7`](https://github.com/logpai/Drain3/blob/0fb6d6e1828838fa05c8f6fa5cda878e02ea79c7/LICENSE.txt) names International Business Machines and the Drain3 project contributors. The full notice is retained in `third_party/logpai-Drain3-LICENSE.txt` to preserve that ancestry.

The Go golden corpus is newly generated synthetic input, not copied customer logs or upstream datasets. Expected traces come from executing the pinned licensed Rust implementation. `bench/parity.py` generates the input and records the source revision in every fixture.

## Dependencies

The shipped Go library imports only the Go standard library. No Rust crate source is vendored or linked into it. The Rust verification program uses the pinned Rust implementation, `serde`, and `serde_json`; `bench/rust/Cargo.lock` records its resolved verification-only dependencies.

The Rust implementation depends on `regex`, `rustc-hash`, and `smallvec`, with optional `serde`. Those crates are used by the reference executable, not translated into the Go library. Their source licenses remain in the Cargo registry or Git checkout. This repository does not distribute compiled reference executables.

The separate Go benchmark module depends on MIT-licensed `github.com/kloudmate/drain3` at `bc42b50af108a7a7ce926a1534d78c9cf9373eed`. It calls that library as a comparator and does not copy its implementation. Its notice is retained in `third_party/kloudmate-drain3-LICENSE.txt`.

The optional local documentation renderer in `docs/visuals` uses tldraw SDK 5.6.0 under its separate development/production license, not Apache-2.0. The full SDK license is retained in `third_party/tldraw-LICENSE.md`. Its npm lockfile pins React, Vite, Playwright, and tldraw assets; those dependencies are not part of the Go module. Published drawings contain only newly invented messages and our diagram shapes. No editor bundle, font binaries, or SDK source is vendored or deployed.

This inventory records source origin and retained notices. It is not a legal opinion about every downstream distribution.
