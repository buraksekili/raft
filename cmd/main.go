package main

import (
	"fmt"

	"github.com/buraksekili/raft"
)

func main() {
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
