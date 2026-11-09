package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	drain3 "github.com/sematext/drain3-go"
)

type request struct {
	Config  json.RawMessage `json:"config"`
	Lines   []string        `json:"lines"`
	Queries []string        `json:"queries"`
}
type step struct {
	ID       uint64        `json:"id"`
	Size     uint64        `json:"size"`
	Template string        `json:"template"`
	Change   drain3.Change `json:"change,omitempty"`
}
type parameter struct {
	Value string `json:"value"`
	Mask  int    `json:"mask"`
}
type parsed struct {
	ID     uint64      `json:"id"`
	Params []parameter `json:"params"`
}
type response struct {
	Steps    []step       `json:"steps"`
	Clusters []step       `json:"clusters"`
	Matches  [][3]uint64  `json:"matches"`
	Parsed   [][3]*parsed `json:"parsed"`
}

func snapshot(c drain3.Cluster) step { return step{ID: c.ID, Size: c.Size, Template: c.Template} }
func run(req request) (response, error) {
	cfg := drain3.DefaultConfig()
	if len(req.Config) > 0 {
		if e := json.Unmarshal(req.Config, &cfg); e != nil {
			return response{}, e
		}
	}
	m, e := drain3.New(cfg)
	if e != nil {
		return response{}, e
	}
	out := response{Steps: []step{}, Clusters: []step{}, Matches: [][3]uint64{}, Parsed: [][3]*parsed{}}
	for _, line := range req.Lines {
		r, e := m.Train(line)
		if e != nil {
			return out, e
		}
		s := snapshot(r.Cluster)
		s.Change = r.Change
		out.Steps = append(out.Steps, s)
	}
	for _, c := range m.Clusters() {
		out.Clusters = append(out.Clusters, snapshot(c))
	}
	f := m.Freeze()
	for _, q := range req.Queries {
		var matches [3]uint64
		var parses [3]*parsed
		for i, strategy := range []drain3.Search{drain3.Never, drain3.Fallback, drain3.Always} {
			c, ok, e := f.MatchWith(q, strategy)
			if e != nil {
				return out, e
			}
			if ok {
				matches[i] = c.ID
			}
			p, ok, e := f.Parse(q, strategy)
			if e != nil {
				return out, e
			}
			if ok {
				r := &parsed{ID: p.Cluster.ID, Params: []parameter{}}
				for _, v := range p.Params {
					r.Params = append(r.Params, parameter{Value: v.Value, Mask: v.MaskIndex})
				}
				parses[i] = r
			}
		}
		out.Matches = append(out.Matches, matches)
		out.Parsed = append(out.Parsed, parses)
	}
	return out, nil
}
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 65536), 64<<20)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	for scanner.Scan() {
		var req request
		if e := json.Unmarshal(scanner.Bytes(), &req); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		out, e := run(req)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		if e := json.NewEncoder(writer).Encode(out); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	}
	if e := scanner.Err(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
