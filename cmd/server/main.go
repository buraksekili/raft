package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/buraksekili/raft"
)

var replica = flag.Int("replica", 0, "the number of replica for the state machine")

func main() {
	flag.Parse()
	if *replica == 0 {
		log.Fatal("unexpected replica count given: ", *replica)
	}

	bootstrap(*replica)
	fmt.Scanln()
}

func bootstrap(n int) []*raft.Server {
	cluster := []*raft.Server{}

	var nodes []string
	for i := 0; i < n; i++ {
		nodes = append(nodes, fmt.Sprintf("300%d", i))
	}

	for _, nodeId := range nodes {
		// Create new server instance and start it.
		s := raft.NewServer(nodeId, nodes)
		if err := s.Start(); err != nil {
			log.Fatal(err)
		}

		cluster = append(cluster, s)
	}

	return cluster
}
