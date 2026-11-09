package drain3

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
)

func newTestMiner(t testing.TB, cfg Config) *Miner {
	t.Helper()
	m, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func trainTest(t testing.TB, m *Miner, line string) Trained {
	t.Helper()
	r, e := m.Train(line)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func matchTest(t testing.TB, m *Matcher, line string) Cluster {
	t.Helper()
	c, ok, e := m.Match(line)
	if e != nil || !ok {
		t.Fatalf("match %q: %v %v", line, ok, e)
	}
	return c
}

func TestOrderedTrainingAndSnapshots(t *testing.T) {
	m := newTestMiner(t, DefaultConfig())
	lines := []string{"user alice signed in", "user bob signed in", "user carol signed in", "error upstream unavailable"}
	expected := []Trained{
		{Cluster: Cluster{ID: 1, Size: 1, Template: lines[0]}, Change: Created},
		{Cluster: Cluster{ID: 1, Size: 2, Template: "user <*> signed in"}, Change: TemplateChanged},
		{Cluster: Cluster{ID: 1, Size: 3, Template: "user <*> signed in"}, Change: Unchanged},
		{Cluster: Cluster{ID: 2, Size: 1, Template: lines[3]}, Change: Created},
	}
	first := trainTest(t, m, lines[0])
	if first != expected[0] {
		t.Fatal(first)
	}
	for i := 1; i < len(lines); i++ {
		if got := trainTest(t, m, lines[i]); got != expected[i] {
			t.Fatalf("%d: %+v != %+v", i, got, expected[i])
		}
	}
	if first.Cluster.Template != lines[0] {
		t.Fatal("snapshot changed")
	}
	frozen := m.Freeze()
	if got := matchTest(t, frozen, "user dave signed in"); got.ID != 1 || got.Size != 3 || got.Template != "user <*> signed in" {
		t.Fatal(got)
	}
	if _, ok, e := frozen.Match("error upstream overloaded"); e != nil || ok {
		t.Fatalf("nonmatch %v %v", ok, e)
	}
	if _, e := m.Train("user eve signed in"); !errors.Is(e, ErrFrozen) {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Clusters(), frozen.Clusters()) || !reflect.DeepEqual(frozen.Clusters(), m.Freeze().Clusters()) {
		t.Fatal("freeze changed clusters")
	}
}

func TestEmptyAndSingleTokenLines(t *testing.T) {
	m := newTestMiner(t, DefaultConfig())
	a := trainTest(t, m, "")
	b := trainTest(t, m, " \t\x1c\u00a0")
	if a.Cluster.ID != b.Cluster.ID || b.Change != Unchanged || b.Cluster.Size != 2 {
		t.Fatal(a, b)
	}
	for _, v := range []string{"one", "one", "two"} {
		trainTest(t, m, v)
	}
	if m.Len() != 3 {
		t.Fatal(m.Clusters())
	}
	f := m.Freeze()
	if c := matchTest(t, f, ""); c.ID != 1 || c.Template != "" {
		t.Fatal(c)
	}
}

func TestConfiguration(t *testing.T) {
	for _, edit := range []func(*Config){func(c *Config) { c.Depth = 2 }, func(c *Config) { c.SimilarityThreshold = math.NaN() }, func(c *Config) { c.SimilarityThreshold = math.Inf(1) }, func(c *Config) { c.SimilarityThreshold = -.1 }, func(c *Config) { c.MaxChildren = 0 }, func(c *Config) { c.MaxClusters = -1 }, func(c *Config) { c.MaxLineBytes = -1 }, func(c *Config) { c.MaxTokens = -1 }, func(c *Config) { c.ParamString = "" }, func(c *Config) { c.ParamString = "x\x1fy" }, func(c *Config) { c.Masking = []MaskingInstruction{{Pattern: "("}} }} {
		cfg := DefaultConfig()
		edit(&cfg)
		if _, e := New(cfg); e == nil {
			t.Fatal("accepted", cfg)
		}
	}
	cfg := DefaultConfig()
	cfg.ExtraDelimiters = []string{"="}
	cfg.Masking = []MaskingInstruction{{Pattern: "[0-9]+", Replacement: "<N>"}}
	m := newTestMiner(t, cfg)
	cfg.ExtraDelimiters[0] = "x"
	cfg.Masking[0].Replacement = "broken"
	returned := m.Config()
	returned.ExtraDelimiters[0] = "y"
	returned.Masking[0].Replacement = "broken"
	if got := trainTest(t, m, "x=123"); got.Cluster.Template != "x <N>" {
		t.Fatal(got)
	}
	frozenCfg := m.Freeze().Config()
	frozenCfg.ExtraDelimiters[0] = "z"
	if got := m.Config().ExtraDelimiters[0]; got != "=" {
		t.Fatal(got)
	}
}

func TestBoundsAreAtomic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxClusters = 1
	cfg.MaxLineBytes = 32
	cfg.MaxTokens = 4
	m := newTestMiner(t, cfg)
	trainTest(t, m, "a b c")
	before := m.Clusters()
	for _, test := range []struct {
		line string
		err  error
	}{{"different length", ErrClusterLimit}, {"012345678901234567890123456789012", ErrLineLimit}, {"a b c d e", ErrTokenLimit}} {
		if _, e := m.Train(test.line); !errors.Is(e, test.err) {
			t.Fatalf("%q: %v", test.line, e)
		}
		if !reflect.DeepEqual(before, m.Clusters()) {
			t.Fatal("failed train changed state")
		}
	}
	if got := trainTest(t, m, "a b d"); got.Change != TemplateChanged || got.Cluster.ID != 1 || got.Cluster.Size != 2 {
		t.Fatal(got)
	}
	f := m.Freeze()
	for _, test := range []struct {
		line string
		err  error
	}{{"012345678901234567890123456789012", ErrLineLimit}, {"a b c d e", ErrTokenLimit}} {
		if _, _, e := f.Match(test.line); !errors.Is(e, test.err) {
			t.Fatal(e)
		}
		if _, _, e := f.Parse(test.line, Never); !errors.Is(e, test.err) {
			t.Fatal(e)
		}
	}
	if _, _, e := f.MatchWith("a b z", Search(255)); !errors.Is(e, ErrSearch) {
		t.Fatal(e)
	}
}

func TestChildrenAndWildcardTies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Depth = 3
	cfg.SimilarityThreshold = 1
	cfg.ParametrizeNumericTokens = false
	m := newTestMiner(t, cfg)
	for _, v := range []string{"a b c", "a <*> c", "a <*> <*>"} {
		trainTest(t, m, v)
	}
	if got := matchTest(t, m.Freeze(), "a b c"); got.Template != "a <*> <*>" {
		t.Fatal(got)
	}
	cfg = DefaultConfig()
	cfg.MaxChildren = 1
	cfg.SimilarityThreshold = 1
	m = newTestMiner(t, cfg)
	for _, v := range []string{"first event happened", "second event happened", "third event happened"} {
		trainTest(t, m, v)
	}
	f := m.Freeze()
	for _, v := range []string{"first", "second", "third"} {
		if got := matchTest(t, f, v+" event happened"); got.Template != v+" event happened" {
			t.Fatal(got)
		}
	}
}

func TestSearchStrategies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SimilarityThreshold = .5
	cfg.MaxChildren = 2
	cfg.ParametrizeNumericTokens = false
	m := newTestMiner(t, cfg)
	for _, v := range []string{"first x end", "second y end", "third z end"} {
		trainTest(t, m, v)
	}
	f := m.Freeze()
	for _, line := range []string{"first x end", "second y end", "third z end"} {
		for _, strategy := range []Search{Never, Fallback, Always} {
			before := f.Clusters()
			_, _, e := f.MatchWith(line, strategy)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(before, f.Clusters()) {
				t.Fatal("matching mutated state")
			}
		}
	}
}

func TestConcurrentTrainFreezeAndMatch(t *testing.T) {
	m := newTestMiner(t, DefaultConfig())
	trainTest(t, m, "request 0 served")
	trainTest(t, m, "request 1 served")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, e := m.Train(fmt.Sprintf("request %d served", i*100+j)); e != nil && !errors.Is(e, ErrFrozen) {
					t.Error(e)
				}
			}
		}(i)
	}
	frozen := m.Freeze()
	wg.Wait()
	before := frozen.Clusters()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_, ok, e := frozen.Match("request anything served")
				if e != nil || !ok {
					t.Errorf("match %v %v", ok, e)
				}
				_, ok, e = frozen.Parse("request anything served", Never)
				if e != nil || !ok {
					t.Errorf("parse %v %v", ok, e)
				}
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(before, frozen.Clusters()) {
		t.Fatal("matching changed model")
	}
}
