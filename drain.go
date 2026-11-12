// Copyright 2026 Akshat Agarwal
// SPDX-License-Identifier: Apache-2.0
//
// Ported and modified from Rust drain3 at 9d5e8e8cad620bc2b83064e8c67e29affccf571c.
// The Go port changes storage, concurrency, resource bounds, and the public API.

package drain3

import (
	"fmt"
	"strings"
	"sync"
)

// Change describes the effect of training one line.
type Change string

const (
	Created         Change = "created"
	TemplateChanged Change = "template_changed"
	Unchanged       Change = "unchanged"
)

// Cluster is an immutable snapshot. IDs start at one and are never reused.
type Cluster struct {
	ID       uint64
	Size     uint64
	Template string
}

// Trained records the assignment and the resulting template snapshot.
type Trained struct {
	Cluster Cluster
	Change  Change
}

type cluster struct {
	Cluster
	tokens []string
	params int
}

type node struct {
	children map[string]*node
	wildcard *node
	ordered  []*node
	clusters []*cluster
}

type model struct {
	cfg      Config
	prep     preparer
	roots    map[int]*node
	clusters []*cluster
}

// Miner serializes operations, but callers must supply training order when
// deterministic clusters are required. Freeze permanently disables training.
// A Miner must not be copied after first use.
type Miner struct {
	mu     sync.RWMutex
	state  *model
	frozen bool
}

// Matcher shares frozen model state without copying it or retaining input lines.
// All its methods may run concurrently. Returned cluster values are snapshots.
type Matcher struct{ state *model }

// New validates a configuration and compiles its masking expressions.
func New(cfg Config) (*Miner, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg = cfg.clone()
	p, err := newPreparer(cfg)
	if err != nil {
		return nil, err
	}
	return &Miner{state: &model{cfg: cfg, prep: p, roots: make(map[int]*node)}}, nil
}

// Config returns a detached configuration copy.
func (m *Miner) Config() Config { m.mu.RLock(); defer m.mu.RUnlock(); return m.state.cfg.clone() }

// Config returns a detached configuration copy.
func (m *Matcher) Config() Config { return m.state.cfg.clone() }

// Len returns the number of clusters.
func (m *Miner) Len() int { m.mu.RLock(); defer m.mu.RUnlock(); return len(m.state.clusters) }

// Len returns the number of clusters.
func (m *Matcher) Len() int { return len(m.state.clusters) }

// Clusters returns snapshots in ascending ID order.
func (m *Miner) Clusters() []Cluster { m.mu.RLock(); defer m.mu.RUnlock(); return m.state.snapshots() }

// Clusters returns snapshots in ascending ID order.
func (m *Matcher) Clusters() []Cluster { return m.state.snapshots() }
func (m *model) snapshots() []Cluster {
	out := make([]Cluster, len(m.clusters))
	for i, c := range m.clusters {
		out[i] = c.Cluster
	}
	return out
}

// Freeze atomically ends training. Repeated calls share the same immutable state.
func (m *Miner) Freeze() *Matcher {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frozen = true
	return &Matcher{state: m.state}
}

// Train prepares and assigns one line, generalizing the winning template.
// A limit error does not consume an ID or modify counts, templates or the tree.
func (m *Miner) Train(line string) (Trained, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.frozen {
		return Trained{}, ErrFrozen
	}
	s := m.state
	_, tokens, err := s.prep.prepare(line, false)
	if err != nil {
		return Trained{}, err
	}
	leaf := s.leaf(tokens)
	var best *cluster
	bestScore := -1
	if leaf != nil {
		for _, c := range leaf.clusters {
			matches := 0
			for i, t := range c.tokens {
				if t != s.cfg.ParamString && t == tokens[i] {
					matches++
				}
			}
			similarity := 1.0
			if len(tokens) > 0 {
				similarity = float64(matches) / float64(len(tokens))
			}
			// An exact template match must remain reusable even when most
			// tokens are parameters. Otherwise repeated normalized lines can
			// consume the entire cluster budget with identical templates.
			if similarity < s.cfg.SimilarityThreshold && !(s.cfg.ReuseExactTemplates && matches+c.params == len(tokens)) {
				continue
			}
			if matches > bestScore || matches == bestScore && best != nil && c.params > best.params {
				best, bestScore = c, matches
			}
		}
	}
	if best != nil {
		changed := false
		for i, t := range best.tokens {
			if t != s.cfg.ParamString && t != tokens[i] {
				best.tokens[i] = s.cfg.ParamString
				best.params++
				changed = true
			}
		}
		best.Size++
		change := Unchanged
		if changed {
			best.Template = strings.Join(best.tokens, " ")
			change = TemplateChanged
		}
		return Trained{Cluster: best.Cluster, Change: change}, nil
	}
	if s.cfg.MaxClusters > 0 && len(s.clusters) >= s.cfg.MaxClusters {
		return Trained{}, ErrClusterLimit
	}
	owned := make([]string, len(tokens))
	params := 0
	for i, t := range tokens {
		owned[i] = strings.Clone(t)
		if t == s.cfg.ParamString {
			params++
		}
	}
	c := &cluster{Cluster: Cluster{ID: uint64(len(s.clusters)) + 1, Size: 1, Template: strings.Join(owned, " ")}, tokens: owned, params: params}
	s.insert(c)
	s.clusters = append(s.clusters, c)
	return Trained{Cluster: c.Cluster, Change: Created}, nil
}

func (s *model) leaf(tokens []string) *node {
	n := s.roots[len(tokens)]
	if n == nil {
		return nil
	}
	limit := min(s.cfg.Depth-3, len(tokens)-1)
	for i := 0; i < limit; i++ {
		next := n.children[tokens[i]]
		if next == nil || tokens[i] == s.cfg.ParamString {
			next = n.wildcard
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

func (s *model) insert(c *cluster) {
	n := s.roots[len(c.tokens)]
	if n == nil {
		n = &node{}
		s.roots[len(c.tokens)] = n
	}
	limit := min(s.cfg.Depth-3, len(c.tokens)-1)
	for i := 0; i < limit; i++ {
		token := c.tokens[i]
		next := n.children[token]
		if token == s.cfg.ParamString {
			next = n.wildcard
		}
		if next == nil {
			count := len(n.children)
			if n.wildcard != nil {
				count++
			}
			wild := token == s.cfg.ParamString || s.cfg.ParametrizeNumericTokens && hasNumbers(token)
			if !wild {
				if n.wildcard != nil {
					wild = count >= s.cfg.MaxChildren
				} else {
					wild = count+1 >= s.cfg.MaxChildren
				}
			}
			if wild {
				next = n.wildcard
				if next == nil {
					next = &node{}
					n.wildcard = next
					n.ordered = append(n.ordered, next)
				}
			} else {
				next = &node{}
				if n.children == nil {
					n.children = make(map[string]*node)
				}
				n.children[strings.Clone(token)] = next
				n.ordered = append(n.ordered, next)
			}
		}
		n = next
	}
	n.clusters = append(n.clusters, c)
}

// Match performs exact matching in the routed leaf without changing the miner.
func (m *Miner) Match(line string) (Cluster, bool, error) { return m.MatchWith(line, Never) }

// MatchWith performs exact matching with an explicit search strategy.
func (m *Miner) MatchWith(line string, strategy Search) (Cluster, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.match(line, strategy)
}

// Match performs exact matching in the routed leaf.
func (m *Matcher) Match(line string) (Cluster, bool, error) { return m.MatchWith(line, Never) }

// MatchWith performs exact matching with an explicit search strategy.
func (m *Matcher) MatchWith(line string, strategy Search) (Cluster, bool, error) {
	return m.state.match(line, strategy)
}

func (s *model) match(line string, strategy Search) (Cluster, bool, error) {
	if strategy > Always {
		return Cluster{}, false, ErrSearch
	}
	_, tokens, err := s.prep.prepare(line, false)
	if err != nil {
		return Cluster{}, false, err
	}
	c := s.find(tokens, strategy)
	if c == nil {
		return Cluster{}, false, nil
	}
	return c.Cluster, true, nil
}

func (s *model) find(tokens []string, strategy Search) *cluster {
	bestExact := func(n *node, best *cluster) *cluster {
		if n == nil {
			return best
		}
		for _, c := range n.clusters {
			if best != nil && c.params <= best.params {
				continue
			}
			matches := true
			for i, t := range c.tokens {
				if t != s.cfg.ParamString && t != tokens[i] {
					matches = false
					break
				}
			}
			if matches {
				best = c
			}
		}
		return best
	}
	if strategy != Always {
		best := bestExact(s.leaf(tokens), nil)
		if best != nil || strategy == Never {
			return best
		}
	}
	var best *cluster
	// Child order is insertion order, including the wildcard child.
	stack := []*node{s.roots[len(tokens)]}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil {
			continue
		}
		best = bestExact(n, best)
		for i := len(n.ordered) - 1; i >= 0; i-- {
			stack = append(stack, n.ordered[i])
		}
	}
	return best
}

// Tokenize applies the same masks, literal delimiters and whitespace rules as Train.
// Returned strings can reference line; the caller controls their lifetime.
func (m *Miner) Tokenize(line string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, t, e := m.state.prep.prepare(line, false)
	return t, e
}

// Tokenize applies the same preparation as training.
func (m *Matcher) Tokenize(line string) ([]string, error) {
	_, t, e := m.state.prep.prepare(line, false)
	return t, e
}

// Param refers to a substring of the original input. MaskIndex is -1 for a
// generalized wildcard, otherwise the index of a configured masking instruction.
type Param struct {
	Value     string
	MaskIndex int
}

// Parsed contains a matched cluster snapshot and parameters in input order.
type Parsed struct {
	Cluster Cluster
	Params  []Param
}

// Parse matches a line and extracts original parameter values.
func (m *Miner) Parse(line string, strategy Search) (Parsed, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.parse(line, strategy)
}

// Parse matches a line and extracts original parameter values concurrently.
func (m *Matcher) Parse(line string, strategy Search) (Parsed, bool, error) {
	return m.state.parse(line, strategy)
}
func (s *model) parse(line string, strategy Search) (Parsed, bool, error) {
	if strategy > Always {
		return Parsed{}, false, ErrSearch
	}
	p, tokens, err := s.prep.prepare(line, true)
	if err != nil {
		return Parsed{}, false, err
	}
	c := s.find(tokens, strategy)
	if c == nil {
		return Parsed{}, false, nil
	}
	out := Parsed{Cluster: c.Cluster}
	at := 0
	for i, t := range tokens {
		offset := strings.Index(p.text[at:], t)
		if offset < 0 {
			return Parsed{}, false, fmt.Errorf("drain3: inconsistent prepared token")
		}
		start := at + offset
		end := start + len(t)
		at = end
		if c.tokens[i] == s.cfg.ParamString {
			out.Params = append(out.Params, Param{Value: p.original(start, end), MaskIndex: -1})
		} else {
			out.Params = p.masked(start, end, out.Params)
		}
	}
	return out, true, nil
}
