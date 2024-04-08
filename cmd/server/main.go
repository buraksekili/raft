package main

import (
	"flag"
	"fmt"

	"github.com/buraksekili/raft"
)

var numberOfNodes = flag.Int("replica", 3, "no of raft server node")
var id = flag.Int("id", 1, "")

func main() {
	flag.Parse()

	var nodeCluster []*raft.Node
	for _, nodeAddr := range getNodeAddrs(*numberOfNodes) {
		n := raft.NewNode(nodeAddr)
		nodeCluster = append(nodeCluster, n)
	}

	for _, n := range nodeCluster {
		n.SetCluster(nodeCluster)
	}

	err := nodeCluster[*id].Start()
	if err != nil {
		panic(err)
	}
}

func getNodeAddrs(n int) (res []string) {
	for i := 0; i < n; i++ {
		res = append(res, fmt.Sprintf("300%d", i))
	}

	return
}
