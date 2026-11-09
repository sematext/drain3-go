package drain3

import (
	"reflect"
	"testing"
)

func FuzzFrozenRoundTrip(f *testing.F) {
	for _, s := range []string{"", "request 123 served", "<*> <*>", "é\x1f四 ٢", "a\x00b\xff c"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if len(line) > 4096 {
			t.Skip()
		}
		m := newTestMiner(t, DefaultConfig())
		trainTest(t, m, line)
		trainTest(t, m, line)
		before := m.Clusters()
		var count uint64
		for i, c := range before {
			if c.ID != uint64(i+1) {
				t.Fatal(c)
			}
			count += c.Size
		}
		if count != 2 {
			t.Fatal(before)
		}
		frozen := m.Freeze()
		for _, strategy := range []Search{Never, Fallback, Always} {
			c, ok, e := frozen.MatchWith(line, strategy)
			if e != nil || !ok {
				t.Fatalf("trained line did not match: %+v %v %v", c, ok, e)
			}
			p, ok, e := frozen.Parse(line, strategy)
			if e != nil || !ok || p.Cluster != c {
				t.Fatalf("parse differs: %+v %+v %v %v", p, c, ok, e)
			}
		}
		if !reflect.DeepEqual(before, frozen.Clusters()) {
			t.Fatal("frozen state changed")
		}
	})
}

func FuzzTrackedMasking(f *testing.F) {
	for _, s := range []string{"", "pair 12:34", "é123\x1f456", "a=b, x=123", "\xff12"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if len(line) > 4096 {
			t.Skip()
		}
		cfg := DefaultConfig()
		cfg.Masking = []MaskingInstruction{{Pattern: `[0-9]+`, Replacement: "<N>"}, {Pattern: `<N>:<N>`, Replacement: "<PAIR>"}}
		cfg.ExtraDelimiters = []string{"=", ", "}
		m := newTestMiner(t, cfg)
		trainTest(t, m, line)
		frozen := m.Freeze()
		_, ok, e := frozen.Parse(line, Never)
		if e != nil || !ok {
			t.Fatalf("tracked preparation differs: %v %v", ok, e)
		}
	})
}
