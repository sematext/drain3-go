# Drain semantics

This reference describes the Go API against Rust commit `9d5e8e8cad620bc2b83064e8c67e29affccf571c`.

## Preparation

Masks run in configuration order. Each rule sees the preceding rule's output. Capture replacement follows Go `regexp` semantics.

Extra delimiters are literal strings, not regular expressions. If every nonempty delimiter is one rune, the tokenizer treats those runes as separators. Otherwise, it replaces delimiters with spaces in configuration order. Empty delimiters do nothing.

Whitespace includes `unicode.IsSpace` and ASCII information separators U+001C through U+001F. Tokens retain their original spelling. Templates join tokens with one ASCII space.

Parameter extraction maps prepared bytes back to original byte positions or masking spans. Generalized tokens return the original substring for the whole token. Fixed tokens return values for the mask spans they contain. These results preserve upstream span order, including overlapping values from sequential masks.

## Tree routing

The first level selects token count. The next `min(Depth - 3, tokenCount - 1)` levels select token prefixes. Routing tries an existing literal child before the wildcard child.

Insertion reserves a child slot for the wildcard. Numeric-token routing uses the wildcard child when enabled. A token equal to `ParamString` also uses that child. Child creation order is retained for full searches.

## Training

Training compares only clusters in the routed leaf. Similarity is the number of matching non-wildcard template tokens divided by token count. Empty templates have similarity one. Wildcards do not contribute to training similarity.

Candidates must meet `SimilarityThreshold`. The winner has the most matching constants. Ties prefer more wildcards, then the earliest cluster inserted into that leaf.

A match increments its count. Unequal constant tokens become wildcards. The tree position does not change when a template generalizes. No match creates a cluster with the next ID and count one.

Literal wildcard-only input can create another cluster even when the same line was previously trained, because its training similarity is zero. Exact matching still recognizes wildcard templates.

## Exact matching

Every constant token must match. Wildcards match one token. Among full matches, more wildcards win. Equal wildcard counts retain the first candidate in search order.

`Never` searches the routed leaf. `Fallback` uses that result if one exists, otherwise searches the token-count subtree. `Always` searches that whole subtree. Full searches visit children in creation order and clusters in leaf insertion order.

Matching does not use the training threshold and does not update learned state.

## Ownership

A mutable miner owns configuration, compiled regexes, tree nodes, and retained token strings. Training clones retained tokens and literal tree keys rather than retaining the original input backing string. Cluster snapshots contain immutable strings.

`Freeze` holds the miner's exclusive lock while setting its permanent frozen state. It returns a matcher that shares the model. Once published, no API path mutates that model. Read-only matcher operations allocate their own preparation and parameter data.

An immutable snapshot copy would duplicate tree storage. A mutable matcher would require synchronization and permit accidental count or template changes. Permanent freeze avoids both costs. It also prohibits further streaming updates to that miner. Use a new miner to train another model.

## Resource policy

The Rust implementation uses slots, interning, and optional LRU eviction. Matching does not refresh LRU recency; training does. The Go model has no eviction, free slots, or token interner. A configured cluster cap rejects creation before mutating the tree or consuming an ID.

This is an intentional policy difference, not LRU equivalence. Bounds and freezing are Go-specific behavior and are tested separately from the Rust golden fixtures.
