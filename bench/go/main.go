package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	legacy "github.com/kloudmate/drain3"
	drain3 "github.com/sematext/drain3-go"
)

type cluster struct {
	Partition uint32 `json:"partition"`
	ID        uint64 `json:"id"`
	Size      uint64 `json:"size"`
	Template  string `json:"template"`
}
type result struct {
	Parser      string    `json:"parser"`
	Rows        int       `json:"rows"`
	TrainNS     int64     `json:"trainNs"`
	MatchNS     int64     `json:"matchNs"`
	TrainDigest uint64    `json:"trainDigest"`
	MatchDigest uint64    `json:"matchDigest"`
	Unmatched   int       `json:"unmatched"`
	Clusters    []cluster `json:"clusters"`
	TrainIDs    []uint64  `json:"trainIds,omitempty"`
	MatchIDs    []uint64  `json:"matchIds,omitempty"`
}

const seed uint64 = 14695981039346656037
const prime uint64 = 1099511628211

func digest(h uint64, p uint32, id uint64) uint64 { return ((h^uint64(p))*prime ^ id) * prime }

func frames(path string, fn func(uint32, string) error) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var head [8]byte
	var body []byte
	for {
		n, e := io.ReadFull(r, head[:])
		if e == io.EOF && n == 0 {
			return nil
		}
		if e != nil {
			return e
		}
		partition, size := binary.LittleEndian.Uint32(head[:4]), binary.LittleEndian.Uint32(head[4:])
		if size > 1<<20 {
			return fmt.Errorf("frame exceeds 1 MiB")
		}
		if cap(body) < int(size) {
			body = make([]byte, size)
		} else {
			body = body[:size]
		}
		if _, e := io.ReadFull(r, body); e != nil {
			return e
		}
		if e := fn(partition, string(body)); e != nil {
			return e
		}
	}
}
func run(path, parser string, numeric, trace bool) (result, error) {
	out := result{Parser: parser, TrainDigest: seed, MatchDigest: seed, Clusters: []cluster{}}
	modern := map[uint32]*drain3.Miner{}
	old := map[uint32]*legacy.Drain{}
	start := time.Now()
	e := frames(path, func(partition uint32, line string) error {
		var id uint64
		switch parser {
		case "go":
			m := modern[partition]
			if m == nil {
				cfg := drain3.DefaultConfig()
				cfg.ParametrizeNumericTokens = numeric
				var e error
				m, e = drain3.New(cfg)
				if e != nil {
					return e
				}
				modern[partition] = m
			}
			r, e := m.Train(line)
			if e != nil {
				return e
			}
			id = r.Cluster.ID
		case "kloudmate":
			m := old[partition]
			if m == nil {
				cfg := legacy.DefaultDrainConfig()
				cfg.ParametrizeNumericTokens = numeric
				m = legacy.NewDrain(cfg)
				old[partition] = m
			}
			c, _ := m.AddLogMessage(line)
			id = uint64(c.ClusterID)
		default:
			return fmt.Errorf("unknown parser %q", parser)
		}
		out.Rows++
		out.TrainDigest = digest(out.TrainDigest, partition, id)
		if trace {
			out.TrainIDs = append(out.TrainIDs, id)
		}
		return nil
	})
	out.TrainNS = time.Since(start).Nanoseconds()
	if e != nil {
		return out, e
	}
	frozen := map[uint32]*drain3.Matcher{}
	for p, m := range modern {
		frozen[p] = m.Freeze()
	}
	for _, m := range old {
		m.SimTh = 1
	}
	start = time.Now()
	e = frames(path, func(partition uint32, line string) error {
		var id uint64
		if parser == "go" {
			c, ok, e := frozen[partition].Match(line)
			if e != nil {
				return e
			}
			if ok {
				id = c.ID
			}
		} else {
			c := old[partition].Match(line, legacy.SearchStrategyNever)
			if c != nil {
				id = uint64(c.ClusterID)
			}
		}
		if id == 0 {
			out.Unmatched++
		}
		out.MatchDigest = digest(out.MatchDigest, partition, id)
		if trace {
			out.MatchIDs = append(out.MatchIDs, id)
		}
		return nil
	})
	out.MatchNS = time.Since(start).Nanoseconds()
	if e != nil {
		return out, e
	}
	for p, m := range frozen {
		for _, c := range m.Clusters() {
			out.Clusters = append(out.Clusters, cluster{Partition: p, ID: c.ID, Size: c.Size, Template: c.Template})
		}
	}
	for p, m := range old {
		for _, c := range m.IDToCluster.Values() {
			out.Clusters = append(out.Clusters, cluster{Partition: p, ID: uint64(c.ClusterID), Size: uint64(c.Size), Template: c.GetTemplate()})
		}
	}
	sort.Slice(out.Clusters, func(i, j int) bool {
		a, b := out.Clusters[i], out.Clusters[j]
		if a.Partition != b.Partition {
			return a.Partition < b.Partition
		}
		return a.ID < b.ID
	})
	return out, nil
}
func main() {
	path := flag.String("input", "", "length-prefixed partition/message frames")
	parser := flag.String("parser", "go", "go or kloudmate")
	numeric := flag.Bool("numeric", false, "route numeric tokens through wildcard children")
	trace := flag.Bool("trace", false, "retain ordered assignment IDs for parity checks")
	flag.Parse()
	out, e := run(*path, *parser, *numeric, *trace)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if e := json.NewEncoder(os.Stdout).Encode(out); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
