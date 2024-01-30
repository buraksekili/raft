package raft

import (
	"fmt"
	"net/rpc"
	"testing"
	"time"

	"github.com/buraksekili/raft/internal"
)

func getNodeAddrs(n int) (res []string) {
	for i := 0; i < n; i++ {
		res = append(res, fmt.Sprintf("30%d%d", n, i))
	}

	return
}

func bootstrapCluster(numberOfNodes int) []*Node {
	var nodeCluster []*Node
	nodeAddrs := getNodeAddrs(numberOfNodes)

	for _, nodeAddr := range nodeAddrs {
		n := NewNode(nodeAddr, nodeAddrs)
		nodeCluster = append(nodeCluster, n)
	}

	for _, node := range nodeCluster {
		go func(n *Node) {
			fmt.Println("starting =>", n.id)
			err := n.Start()
			fmt.Printf("stop running %v, due to %v\n", n.id, err)
			return
		}(node)
	}

	return nodeCluster
}

func TestLeaderElection(t *testing.T) {
	tests := []struct {
		name      string
		nodeCount int
	}{
		{
			name:      "leader election in single node cluster",
			nodeCount: 1,
		},
		{
			name:      "leader election with two nodes",
			nodeCount: 2,
		},
		{
			name:      "leader election with three nodes",
			nodeCount: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := bootstrapCluster(test.nodeCount)

			t.Cleanup(func() {
				t.Log("shutting down attempt")
				err := shutdown(cluster)
				t.Logf("shutting down with %v", err)
			})

			leaderCount := 0
			err := internal.RetryGeneric(6, 200*time.Millisecond, func() (*rpc.Client, error) {
				leaderCount = 0
				for _, node := range cluster {
					if node.state == leaderState {
						leaderCount++
					}
				}

				if leaderCount != 1 {
					return nil, fmt.Errorf("leader count expected %v got %v", 1, leaderCount)
				}

				return nil, nil
			})
			if err != nil {
				t.Fatalf("failed to get correct number of leaders %v", err)
			}

		})
	}

}

func shutdown(cluster []*Node) error {
	for _, n := range cluster {
		err := internal.RetryGeneric(3, 500*time.Millisecond,
			func() (*rpc.Client, error) {
				if err := n.CloseServer(); err != nil {
					fmt.Printf("failed to disconnect %v, err %v\n", n.id, err)
					return nil, err
				}

				return nil, nil
			})
		if err != nil {
			return err
		}
	}

	return internal.RetryGeneric(5, 500*time.Millisecond, func() (*rpc.Client, error) {
		for _, n := range cluster {
			if n.running != false {
				return nil, fmt.Errorf("node: %v is still running", n.id)
			}
			if n.listener != nil {
				return nil, fmt.Errorf("node: %v has valid listener", n.id)
			}
		}
		return nil, nil
	})
}
