package main

import (
	"fmt"
	_ "net/http/pprof"

	"github.com/buraksekili/raft"
	"github.com/pkg/profile"
)

func main() {
	defer profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.ProfilePath(".")).Stop()

	var nodeCluster []*raft.Node
	nodeAddrs := []string{"3000", "3001", "3002"}

	for _, nodeAddr := range nodeAddrs {
		n := raft.NewNode(nodeAddr, nodeAddrs)
		nodeCluster = append(nodeCluster, n)
	}

	ch := make(chan error)

	for _, node := range nodeCluster {
		go func(n *raft.Node) {
			if err := n.Start(); err != nil {
				ch <- err
			}
		}(node)
	}

	fmt.Println("failed to start node, err: ", <-ch)
}
