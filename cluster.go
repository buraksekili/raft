package raft

import (
	"fmt"
	"sync"
)

type Cluster struct {
	Nodes []*Node
}

func (c *Cluster) Start() {
	errc := make(chan error)

	wg := &sync.WaitGroup{}
	for _, node := range c.Nodes {
		wg.Add(1)
		go func(n *Node) {
			n.nodes = c.Nodes
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
