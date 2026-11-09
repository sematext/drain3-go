package drain3

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWhitespaceAndLiteralDelimiters(t *testing.T) {
	for _, test := range []struct {
		extra []string
		line  string
		want  []string
	}{
		{nil, "a\x1fb\u00a0c\u3000d  e", []string{"a", "b", "c", "d", "e"}},
		{[]string{"_", "é"}, "a_béx c", []string{"a", "b", "x", "c"}},
		{[]string{", ", "="}, "k=v1, x=2", []string{"k", "v1", "x", "2"}},
		{[]string{"a", "ab"}, "abc", []string{"bc"}},
		{[]string{"ab", "a"}, "abc", []string{"c"}},
		{[]string{".", ""}, "a.b", []string{"a", "b"}},
	} {
		cfg := DefaultConfig()
		cfg.ExtraDelimiters = test.extra
		m := newTestMiner(t, cfg)
		got, e := m.Tokenize(test.line)
		if e != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%q: %v %v != %v", test.line, got, e, test.want)
		}
	}
}

func TestMasksAndOriginalParameters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Masking = []MaskingInstruction{{Pattern: `[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`, Replacement: "<IP>"}, {Pattern: `(^|[^\w])(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)`, Replacement: "${1}<NUM>"}}
	m := newTestMiner(t, cfg)
	trainTest(t, m, "worker thread-0 code=-1 from 10.0.0.1 took 42.5ms on user_0")
	got := trainTest(t, m, "worker thread-3 code=-7 from 10.0.0.2 took 13.0ms on user_0")
	wantTemplate := "worker thread-<NUM> code=<NUM> from <IP> took <NUM>ms on user_0"
	if got.Cluster.Template != wantTemplate || m.Len() != 1 {
		t.Fatal(got, m.Clusters())
	}
	f := m.Freeze()
	parsed, ok, e := f.Parse("worker thread-9 code=-2 from 10.0.0.8 took 88.0ms on user_0", Never)
	want := []Param{{Value: "9", MaskIndex: 1}, {Value: "-2", MaskIndex: 1}, {Value: "10.0.0.8", MaskIndex: 0}, {Value: "88.0", MaskIndex: 1}}
	if e != nil || !ok || !reflect.DeepEqual(parsed.Params, want) {
		t.Fatalf("%+v %v %v", parsed, ok, e)
	}
}

func TestSequentialMasksPreserveUpstreamSpanOrder(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Masking = []MaskingInstruction{{Pattern: `[0-9]+`, Replacement: "<NUM>"}, {Pattern: `<NUM>:<NUM>`, Replacement: "<PAIR>"}}
	m := newTestMiner(t, cfg)
	trainTest(t, m, "pair 12:34 item 5ms stage a")
	trainTest(t, m, "pair 56:78 item 7s stage b")
	parsed, ok, e := m.Freeze().Parse("pair 90:12 item 6h stage c", Never)
	want := []Param{{Value: "90", MaskIndex: 0}, {Value: "90:12", MaskIndex: 1}, {Value: "12", MaskIndex: 0}, {Value: "6h", MaskIndex: -1}, {Value: "c", MaskIndex: -1}}
	if e != nil || !ok || !reflect.DeepEqual(parsed.Params, want) {
		t.Fatalf("%+v %v %v", parsed, ok, e)
	}
}

func TestDelimiterParameterMapping(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExtraDelimiters = []string{", ", "="}
	m := newTestMiner(t, cfg)
	trainTest(t, m, "k=v1, x=1")
	trainTest(t, m, "k=v2, x=2")
	p, ok, e := m.Freeze().Parse("k=abc, x=99", Never)
	want := []Param{{Value: "abc", MaskIndex: -1}, {Value: "99", MaskIndex: -1}}
	if e != nil || !ok || !reflect.DeepEqual(p.Params, want) {
		t.Fatal(p, ok, e)
	}
}

func TestUnicodeMaskContext(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Masking = []MaskingInstruction{{Pattern: `(é)(\d+)`, Replacement: "${1}<N>"}}
	m := newTestMiner(t, cfg)
	trainTest(t, m, "x é123 y")
	p, ok, e := m.Freeze().Parse("x é456 y", Never)
	if e != nil || !ok || !reflect.DeepEqual(p.Params, []Param{{Value: "456", MaskIndex: 0}}) {
		t.Fatal(p, ok, e)
	}
}

func TestPreparedBoundsAndMaskExpansion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxLineBytes = 8
	cfg.Masking = []MaskingInstruction{{Pattern: "a", Replacement: "long-replacement"}}
	m := newTestMiner(t, cfg)
	if _, e := m.Train("a"); !errors.Is(e, ErrLineLimit) {
		t.Fatal(e)
	}
	if m.Len() != 0 {
		t.Fatal(m.Clusters())
	}
	cfg.Masking = []MaskingInstruction{{Pattern: "(a+)", Replacement: "$1$1$1"}}
	m = newTestMiner(t, cfg)
	if _, e := m.Train("aaaa"); !errors.Is(e, ErrLineLimit) {
		t.Fatal(e)
	}
	cfg = DefaultConfig()
	cfg.MaxTokens = 2
	m = newTestMiner(t, cfg)
	if _, e := m.Train(strings.Repeat("word ", 100000)); !errors.Is(e, ErrTokenLimit) {
		t.Fatal(e)
	}
	if m.Len() != 0 {
		t.Fatal(m.Clusters())
	}
}

func TestEmptyMatchesAndRemovedMasks(t *testing.T) {
	for _, test := range []struct{ pattern, replacement, line string }{
		{"a*", "X", "aba"}, {"", "-", "éabc"}, {"[0-9]+", "", "x 12 y"}, {"(foo)", "${1}", "foo bar"}, {"a", "$$", "a b"},
	} {
		cfg := DefaultConfig()
		cfg.Masking = []MaskingInstruction{{Pattern: test.pattern, Replacement: test.replacement}}
		m := newTestMiner(t, cfg)
		untracked, e := m.Tokenize(test.line)
		if e != nil {
			t.Fatal(e)
		}
		trainTest(t, m, test.line)
		p, ok, e := m.Freeze().Parse(test.line, Never)
		if e != nil || !ok || p.Cluster.Template != strings.Join(untracked, " ") {
			t.Fatalf("%+v %v %v", p, ok, e)
		}
	}
}
