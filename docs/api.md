# API and bounds

Import as `drain3 "github.com/sematext/drain3-go"`. Requires Go 1.23+. See the [runnable example](../example_test.go).

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

## Methods and ownership

- `Train` returns a cluster snapshot and `Created`, `TemplateChanged`, or `Unchanged`.
- IDs start at one. `Clusters` returns snapshots in ID order. Returned values do not change after further training.
- `Match` requires an exact match of every non-wildcard token and searches only the routed leaf.
- `MatchWith` and `Parse` accept `Never`, `Fallback`, or `Always`. `Fallback` scans all same-length clusters only after a tree miss. `Always` searches those clusters in tree insertion order.
- `Parse` returns original parameter values. `MaskIndex == -1` identifies a generalized wildcard. Other indices identify masking instructions.
- `Tokenize` applies masks, literal delimiters, and whitespace splitting. Its strings may reference the input. The learned model clones retained tokens and tree keys.
- A miner serializes its methods. Concurrent callers must establish training order themselves when determinism matters. A miner must not be copied after use.
- `Freeze` is permanent and idempotent. Further `Train` calls return `ErrFrozen`. Matcher methods can run concurrently without locks.

## Bounds

`MaxClusters`, `MaxLineBytes`, and `MaxTokens` default to zero: unlimited. Configure them for untrusted input.

A failure returns `ErrClusterLimit`, `ErrLineLimit`, or `ErrTokenLimit` without changing learned state or consuming IDs. Existing clusters can still learn at the cluster cap. No silent eviction or truncation occurs.

`MaxLineBytes` applies to the original line and each completed mask/delimiter stage. Over-limit replacements fail. `MaxTokens` rejects an over-limit line before allocating its token slice. These are data-size bounds, not an RSS limit. Regex capture expansion, runtime overhead, caller-owned buffers, preprocessing, and concurrency can require additional memory.

## Compatibility

The baseline is Rust commit `9d5e8e8cad620bc2b83064e8c67e29affccf571c`. [Detailed semantics](semantics.md) describe routing, similarity, tie-breaking, and ownership.

- Exact matching prefers **more wildcards** on ties, then the first candidate in search order. This can combine distinctions retained by Kloudmate, whose exact ties prefer fewer wildcards. Fewer groups are not evidence of greater accuracy.
- Rust's optional cluster cap uses LRU eviction; this API returns an error instead.
- Go `regexp` is not Rust `regex`. Go `\d`, `\w`, and `\s` are ASCII classes. Use explicit portable Unicode classes where needed. Syntax is not translated.
- Numeric classification and whitespace use Go's Unicode tables; versions can differ from Rust.
- Sequential masks preserve upstream span behavior. Reused marker prefixes/suffixes can produce overlapping original parameter values. Masking numbers to `<N>` and then `<N>:<N>` to `<PAIR>` can report both number values and the pair.
- Snapshot loading, persistence, LRU state, and token interning are not ported. JSON configuration tags do not imply snapshot compatibility.
- Go strings can contain arbitrary bytes; the Rust reference accepts UTF-8 only.
