// Copyright 2026 Akshat Agarwal
// SPDX-License-Identifier: Apache-2.0
//
// Ported and modified from Rust drain3 at 9d5e8e8cad620bc2b83064e8c67e29affccf571c.
// The Go port adds explicit bounds and frozen model ownership.

// Package drain3 mines log templates with a streaming Drain prefix tree.
// Training is ordered and mutable. Freeze publishes a concurrent read-only matcher.
package drain3

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

// Search selects the scope of exact matching. Training always uses the tree.
type Search uint8

const (
	// Never searches only the routed tree leaf.
	Never Search = iota
	// Fallback scans all same-length clusters only when tree matching fails.
	Fallback
	// Always scans all same-length clusters in tree insertion order.
	Always
)

var (
	ErrFrozen       = errors.New("drain3: miner is frozen")
	ErrClusterLimit = errors.New("drain3: cluster limit exceeded")
	ErrLineLimit    = errors.New("drain3: line byte limit exceeded")
	ErrTokenLimit   = errors.New("drain3: token limit exceeded")
	ErrSearch       = errors.New("drain3: invalid search strategy")
)

// MaskingInstruction replaces matches before delimiter processing and tokenization.
// Pattern and replacement use Go regexp syntax, including capture expansion.
type MaskingInstruction struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"mask"`
}

// Config controls training and bounds. Zero limits are unlimited. Bounds reject
// operations without changing learned state; clusters are never evicted.
type Config struct {
	Depth                    int                  `json:"depth"`
	SimilarityThreshold      float64              `json:"sim_threshold"`
	MaxChildren              int                  `json:"max_children"`
	MaxClusters              int                  `json:"max_clusters"`
	MaxLineBytes             int                  `json:"max_line_bytes"`
	MaxTokens                int                  `json:"max_tokens"`
	ExtraDelimiters          []string             `json:"extra_delimiters"`
	ParamString              string               `json:"param_str"`
	ParametrizeNumericTokens bool                 `json:"parametrize_numeric_tokens"`
	Masking                  []MaskingInstruction `json:"masking"`
}

// DefaultConfig matches the Rust upstream's unlimited training defaults.
func DefaultConfig() Config {
	return Config{Depth: 4, SimilarityThreshold: .4, MaxChildren: 100, ParamString: "<*>", ParametrizeNumericTokens: true}
}

func (c Config) validate() error {
	switch {
	case c.Depth < 3:
		return fmt.Errorf("drain3: depth must be at least 3")
	case math.IsNaN(c.SimilarityThreshold) || c.SimilarityThreshold < 0 || c.SimilarityThreshold > 1:
		return fmt.Errorf("drain3: similarity threshold must be in [0,1]")
	case c.MaxChildren < 1:
		return fmt.Errorf("drain3: max children must be positive")
	case c.MaxClusters < 0 || c.MaxLineBytes < 0 || c.MaxTokens < 0:
		return fmt.Errorf("drain3: limits must be nonnegative")
	case c.ParamString == "" || strings.ContainsFunc(c.ParamString, isSpace):
		return fmt.Errorf("drain3: parameter string must be nonempty without whitespace")
	}
	return nil
}

func (c Config) clone() Config {
	c.ExtraDelimiters = append([]string(nil), c.ExtraDelimiters...)
	c.Masking = append([]MaskingInstruction(nil), c.Masking...)
	return c
}

func isSpace(r rune) bool      { return unicode.IsSpace(r) || r >= '\x1c' && r <= '\x1f' }
func hasNumbers(s string) bool { return strings.ContainsFunc(s, unicode.IsNumber) }
