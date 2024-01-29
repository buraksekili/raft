package main

import (
	"flag"
	"fmt"
	"sync"

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

	errc := make(chan error)

	wg := &sync.WaitGroup{}
	for _, node := range nodeCluster {
		wg.Add(1)
		go func(n *raft.Node) {
			err := n.Start()
			wg.Done()
			if err != nil {
				errc <- err
			}
		}(node)
	}

	go func() {
		for {
			select {
			case err := <-errc:
				fmt.Println("err: ", err)
			}
		}
	}()

	wg.Wait()
}

func getNodeAddrs(n int) (res []string) {
	for i := 0; i < n; i++ {
		res = append(res, fmt.Sprintf("300%d", i))
	}

	return
}
