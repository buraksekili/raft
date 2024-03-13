package main

import (
	"flag"
	"fmt"

	"github.com/buraksekili/raft"
	"github.com/pkg/profile"
)

var (
	numberOfNodes   = flag.Int("replica", 1, "no of raft server node")
	enableProfiling = flag.Bool("profiling", false, "enables mem profiling")
)

func main() {
	flag.Parse()

	if *enableProfiling {
		defer profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.ProfilePath(".")).Stop()
	}
	if *numberOfNodes == 0 {
		fmt.Println("Running Raft with replica: ", *numberOfNodes)
		*numberOfNodes = 1
	}

	var nodeCluster []*raft.Node
	nodeAddrs := getNodeAddrs(*numberOfNodes)

	for _, nodeAddr := range nodeAddrs {
		n := raft.NewNode(nodeAddr, nodeAddrs)
		nodeCluster = append(nodeCluster, n)
	}

	c := raft.Cluster{Nodes: nodeCluster}
	c.Start()
}

func getNodeAddrs(n int) (res []string) {
	for i := 0; i < n; i++ {
		res = append(res, fmt.Sprintf("300%d", i))
	}

	return
}
