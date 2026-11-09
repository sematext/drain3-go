package drain3_test

import (
	"fmt"
	"log"

	drain3 "github.com/sematext/drain3-go"
)

func ExampleMiner_Freeze() {
	cfg := drain3.DefaultConfig()
	cfg.MaxClusters = 1000
	cfg.MaxLineBytes = 4096
	cfg.MaxTokens = 256
	miner, err := drain3.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	for _, line := range []string{"user alice signed in", "user bob signed in"} {
		if _, err := miner.Train(line); err != nil {
			log.Fatal(err)
		}
	}
	matcher := miner.Freeze()
	cluster, found, err := matcher.Match("user carol signed in")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(found, cluster.ID, cluster.Size, cluster.Template)
	// Output: true 1 2 user <*> signed in
}
