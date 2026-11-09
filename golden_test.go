package drain3_test

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	drain3 "github.com/sematext/drain3-go"
)

type goldenStep struct {
	ID       uint64        `json:"id"`
	Size     uint64        `json:"size"`
	Template string        `json:"template"`
	Change   drain3.Change `json:"change,omitempty"`
}
type goldenParam struct {
	Value string `json:"value"`
	Mask  int    `json:"mask"`
}
type goldenParsed struct {
	ID     uint64        `json:"id"`
	Params []goldenParam `json:"params"`
}
type goldenResponse struct {
	Steps    []goldenStep       `json:"steps"`
	Clusters []goldenStep       `json:"clusters"`
	Matches  [][3]uint64        `json:"matches"`
	Parsed   [][3]*goldenParsed `json:"parsed"`
}
type goldenCase struct {
	Upstream string `json:"upstream"`
	Name     string `json:"name"`
	Request  struct {
		Config  json.RawMessage `json:"config"`
		Lines   []string        `json:"lines"`
		Queries []string        `json:"queries"`
	} `json:"request"`
	Expected goldenResponse `json:"expected"`
}

func TestRustGolden(t *testing.T) {
	f, e := os.Open("testdata/rust-golden.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 65536), 1<<20)
	count := 0
	for scanner.Scan() {
		var tc goldenCase
		if e := json.Unmarshal(scanner.Bytes(), &tc); e != nil {
			t.Fatal(e)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Upstream != "9d5e8e8cad620bc2b83064e8c67e29affccf571c" {
				t.Fatal("unexpected upstream", tc.Upstream)
			}
			cfg := drain3.DefaultConfig()
			if e := json.Unmarshal(tc.Request.Config, &cfg); e != nil {
				t.Fatal(e)
			}
			m, e := drain3.New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			actual := goldenResponse{Steps: []goldenStep{}, Clusters: []goldenStep{}, Matches: [][3]uint64{}, Parsed: [][3]*goldenParsed{}}
			for _, line := range tc.Request.Lines {
				r, e := m.Train(line)
				if e != nil {
					t.Fatal(e)
				}
				actual.Steps = append(actual.Steps, goldenStep{ID: r.Cluster.ID, Size: r.Cluster.Size, Template: r.Cluster.Template, Change: r.Change})
			}
			for _, c := range m.Clusters() {
				actual.Clusters = append(actual.Clusters, goldenStep{ID: c.ID, Size: c.Size, Template: c.Template})
			}
			frozen := m.Freeze()
			for _, line := range tc.Request.Queries {
				var matches [3]uint64
				var parses [3]*goldenParsed
				for i, s := range []drain3.Search{drain3.Never, drain3.Fallback, drain3.Always} {
					c, ok, e := frozen.MatchWith(line, s)
					if e != nil {
						t.Fatal(e)
					}
					if ok {
						matches[i] = c.ID
					}
					p, ok, e := frozen.Parse(line, s)
					if e != nil {
						t.Fatal(e)
					}
					if ok {
						parsed := &goldenParsed{ID: p.Cluster.ID, Params: []goldenParam{}}
						for _, v := range p.Params {
							parsed.Params = append(parsed.Params, goldenParam{Value: v.Value, Mask: v.MaskIndex})
						}
						parses[i] = parsed
					}
				}
				actual.Matches = append(actual.Matches, matches)
				actual.Parsed = append(actual.Parsed, parses)
			}
			if !reflect.DeepEqual(actual, tc.Expected) {
				a, _ := json.Marshal(actual)
				want, _ := json.Marshal(tc.Expected)
				t.Fatalf("trace differs\nactual: %s\nexpected: %s", a, want)
			}
		})
	}
	if e := scanner.Err(); e != nil {
		t.Fatal(e)
	}
	if count != 143 {
		t.Fatalf("golden count %d", count)
	}
}
