// Copyright 2026 Akshat Agarwal
// SPDX-License-Identifier: Apache-2.0
//
// Ported and modified from Rust drain3 at 9d5e8e8cad620bc2b83064e8c67e29affccf571c.
// The Go port uses Go regexp syntax and adds explicit preparation bounds.

package drain3

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type rule struct {
	re          *regexp.Regexp
	replacement string
}
type preparer struct {
	cfg     Config
	rules   []rule
	single  map[rune]bool
	general []string
}
type span struct{ start, end, rule int }
type prepared struct {
	input, text string
	source      []int
	spans       []span
}

func newPreparer(cfg Config) (preparer, error) {
	p := preparer{cfg: cfg}
	for _, r := range cfg.Masking {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return p, fmt.Errorf("drain3: invalid masking regex %q: %w", r.Pattern, err)
		}
		p.rules = append(p.rules, rule{re: re, replacement: r.Replacement})
	}
	singles := true
	for _, d := range cfg.ExtraDelimiters {
		if utf8.RuneCountInString(d) > 1 {
			singles = false
		}
	}
	if singles {
		for _, d := range cfg.ExtraDelimiters {
			if d != "" {
				if p.single == nil {
					p.single = make(map[rune]bool)
				}
				r, _ := utf8.DecodeRuneInString(d)
				p.single[r] = true
			}
		}
	} else {
		for _, d := range cfg.ExtraDelimiters {
			if d != "" {
				p.general = append(p.general, d)
			}
		}
	}
	return p, nil
}

func (p *preparer) checkBytes(n int) error {
	if p.cfg.MaxLineBytes > 0 && n > p.cfg.MaxLineBytes {
		return ErrLineLimit
	}
	return nil
}

func (p *preparer) prepare(input string, track bool) (prepared, []string, error) {
	out := prepared{input: input, text: input}
	if err := p.checkBytes(len(input)); err != nil {
		return out, nil, err
	}
	for i, r := range p.rules {
		matches := r.re.FindAllStringSubmatchIndex(out.text, -1)
		if len(matches) == 0 {
			continue
		}
		var b strings.Builder
		var source []int
		last := 0
		for _, m := range matches {
			replacement := r.replacement
			if strings.Contains(replacement, "$") {
				replacement = string(r.re.ExpandString(nil, replacement, out.text, m))
			}
			if err := p.checkBytes(b.Len() + m[0] - last); err != nil {
				return out, nil, err
			}
			b.WriteString(out.text[last:m[0]])
			if track {
				source = out.copySource(source, last, m[0])
			}
			pre, suf := trimContext(out.text[m[0]:m[1]], replacement)
			if err := p.checkBytes(b.Len() + len(replacement)); err != nil {
				return out, nil, err
			}
			b.WriteString(replacement)
			if track {
				source = out.copySource(source, m[0], m[0]+pre)
				middle := len(replacement) - pre - suf
				if middle > 0 {
					start, end := m[0]+pre, m[1]-suf
					os, oe := 0, 0
					if start < end {
						os, oe = out.start(start), out.end(end-1)
					} else if start < len(out.text) {
						os = out.start(start)
						oe = os
					} else if start > 0 {
						os = out.end(start - 1)
						oe = os
					}
					out.spans = append(out.spans, span{start: os, end: max(os, oe), rule: i})
					id := -len(out.spans)
					for j := 0; j < middle; j++ {
						source = append(source, id)
					}
				}
				source = out.copySource(source, m[1]-suf, m[1])
			}
			last = m[1]
		}
		if err := p.checkBytes(b.Len() + len(out.text) - last); err != nil {
			return out, nil, err
		}
		b.WriteString(out.text[last:])
		if track {
			source = out.copySource(source, last, len(out.text))
		}
		out.text = b.String()
		out.source = source
	}
	for _, d := range p.general {
		if !strings.Contains(out.text, d) {
			continue
		}
		var b strings.Builder
		var source []int
		last := 0
		for {
			rel := strings.Index(out.text[last:], d)
			if rel < 0 {
				break
			}
			at := last + rel
			if err := p.checkBytes(b.Len() + at - last + 1); err != nil {
				return out, nil, err
			}
			b.WriteString(out.text[last:at])
			b.WriteByte(' ')
			if track {
				source = out.copySource(source, last, at)
				source = out.copySource(source, at, at+1)
			}
			last = at + len(d)
		}
		if err := p.checkBytes(b.Len() + len(out.text) - last); err != nil {
			return out, nil, err
		}
		b.WriteString(out.text[last:])
		if track {
			source = out.copySource(source, last, len(out.text))
		}
		out.text = b.String()
		out.source = source
	}
	if p.cfg.MaxTokens > 0 {
		count, inside := 0, false
		for _, r := range out.text {
			separator := isSpace(r) || p.single[r]
			if !separator && !inside {
				count++
				if count > p.cfg.MaxTokens {
					return out, nil, ErrTokenLimit
				}
			}
			inside = !separator
		}
	}
	var tokens []string
	if len(p.single) == 0 && !strings.ContainsAny(out.text, "\x1c\x1d\x1e\x1f") {
		tokens = strings.Fields(out.text)
	} else {
		tokens = strings.FieldsFunc(out.text, func(r rune) bool { return isSpace(r) || p.single[r] })
	}
	return out, tokens, nil
}

func (p *prepared) copySource(dst []int, start, end int) []int {
	if p.source != nil {
		return append(dst, p.source[start:end]...)
	}
	for i := start; i < end; i++ {
		dst = append(dst, i)
	}
	return dst
}
func (p *prepared) start(i int) int {
	if p.source == nil {
		return i
	}
	v := p.source[i]
	if v < 0 {
		return p.spans[-v-1].start
	}
	return v
}
func (p *prepared) end(i int) int {
	if p.source == nil {
		return i + 1
	}
	v := p.source[i]
	if v < 0 {
		return p.spans[-v-1].end
	}
	return v + 1
}
func (p *prepared) original(start, end int) string {
	if start == end {
		return ""
	}
	s, e := p.start(start), p.end(end-1)
	return p.input[s:max(s, e)]
}
func (p *prepared) masked(start, end int, out []Param) []Param {
	if p.source == nil {
		return out
	}
	previous := 0
	for _, v := range p.source[start:end] {
		if v < 0 && v != previous {
			s := p.spans[-v-1]
			out = append(out, Param{Value: p.input[s.start:s.end], MaskIndex: s.rule})
		}
		previous = v
	}
	return out
}

func trimContext(a, b string) (int, int) {
	pre := 0
	for pre < min(len(a), len(b)) && a[pre] == b[pre] {
		pre++
	}
	for pre > 0 && pre < len(a) && !utf8.RuneStart(a[pre]) {
		pre--
	}
	suf := 0
	for suf < min(len(a)-pre, len(b)-pre) && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	for suf > 0 && !utf8.RuneStart(a[len(a)-suf]) {
		suf--
	}
	return pre, suf
}
