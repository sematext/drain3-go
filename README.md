# drain3-go

A streaming Go port of [Akshat Agarwal's Apache-2.0 Rust Drain implementation](https://github.com/akshatagarwl/drain3/tree/9d5e8e8cad620bc2b83064e8c67e29affccf571c). The Go library has no third-party dependencies. Its API is experimental.

The miner learns templates in input order. `Freeze` ends training and publishes a read-only matcher without copying the tree. Matching never changes cluster counts or templates.

```go
cfg := drain3.DefaultConfig()
cfg.MaxClusters = 1000
cfg.MaxLineBytes = 4096
cfg.MaxTokens = 256
miner, err := drain3.New(cfg)
if err != nil {
	return err
}
for _, line := range []string{"user alice signed in", "user bob signed in"} {
	if _, err := miner.Train(line); err != nil {
		return err
	}
}
matcher := miner.Freeze()
cluster, found, err := matcher.Match("user carol signed in")
// cluster.Template == "user <*> signed in", found == true, err == nil.
```

Import the package as `drain3 "github.com/sematext/drain3-go"`. Go 1.23 or later is required.

## API reference

- `Train` returns a cluster snapshot and `Created`, `TemplateChanged`, or `Unchanged`.
- IDs start at one. `Clusters` returns snapshots in ID order. Returned values do not change after further training.
- `Match` requires an exact match of every non-wildcard token and searches only the routed leaf.
- `MatchWith` and `Parse` accept `Never`, `Fallback`, or `Always`. `Fallback` scans all same-length clusters only after a tree miss. `Always` searches those clusters in tree insertion order.
- `Parse` returns original parameter values. `MaskIndex == -1` identifies a generalized wildcard. Other indices identify masking instructions.
- `Tokenize` applies masks, literal delimiters, and whitespace splitting. Its strings may reference the input. The learned model clones retained tokens and tree keys.
- A miner serializes its methods. Concurrent callers must establish training order themselves when determinism matters. A miner must not be copied after use.
- `Freeze` is permanent and idempotent. Further `Train` calls return `ErrFrozen`. Matcher methods can run concurrently without locks.

## Bounds reference

`MaxClusters`, `MaxLineBytes`, and `MaxTokens` default to zero, which means unlimited. Configure them when inputs are untrusted.

A bound failure returns `ErrClusterLimit`, `ErrLineLimit`, or `ErrTokenLimit` without changing learned state. IDs are not consumed. Existing clusters can still learn when the cluster cap is full. No cluster is silently evicted, and no line or token list is silently truncated.

`MaxLineBytes` applies to the original line and each completed masking or delimiter stage. Mask replacements that exceed the limit fail. `MaxTokens` rejects an over-limit line before allocating its token slice. These are data-size bounds, not a process RSS limit. Regex capture expansion, runtime overhead, caller-owned input buffers, preprocessing, and concurrent calls can require additional memory.

## Compatibility reference

The baseline is Rust commit `9d5e8e8cad620bc2b83064e8c67e29affccf571c`. [The semantic map](docs/semantics.md) records matching and tie-breaking rules.

Deliberate differences and limitations are:

- Rust's optional cluster cap uses LRU eviction. This API returns an explicit error instead.
- Regex patterns and capture expansion use Go `regexp`, not Rust `regex`. In particular, Go `\d`, `\w`, and `\s` are ASCII classes. Use portable explicit Unicode classes when required. Regex syntax is not translated.
- Numeric classification and whitespace use Go's Unicode tables. Unicode versions can differ from the Rust compiler's tables.
- Sequential masks preserve upstream parameter-span behavior. If masks reuse marker prefixes or suffixes, `Parse` can return overlapping original values from more than one rule. For example, masking numbers to `<N>` and then `<N>:<N>` to `<PAIR>` can report the two number values as well as the pair value.
- Rust snapshot loading, persistence, LRU state, and token interning are not ported. JSON configuration tags do not imply snapshot compatibility.
- The library operates on Go strings. The Rust reference accepts UTF-8 strings only.

## Verification

Verification is self-verification, not an independent review.

```sh
go test -race ./...
go vet ./...
```

The checked-in Rust golden fixtures cover 143 synthetic cases, 10,322 ordered training steps, and 2,210 queries across all three search strategies. They compare assignments, IDs, counts, template changes, final templates, matches, and original parameter extraction. Unit and fuzz tests cover bounds, ownership, and freeze concurrency.

[Reproduce the Rust comparison](bench/README.md). Only synthetic fixtures belong in this repository. Keep customer inputs, templates, and private benchmark artifacts outside it.

## License

Apache-2.0. [Provenance](PROVENANCE.md), [NOTICE](NOTICE), and [third-party notices](third_party) preserve upstream attribution.
