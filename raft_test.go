package raft

import (
	"fmt"
	"net/rpc"
	"testing"
	"time"

	"github.com/buraksekili/raft/internal"
)

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
			t.Log("bootstrapping cluster")
			cluster := bootstrapCluster(test.nodeCount)
			t.Log("bootstrapping cluster done")

			t.Cleanup(func() {
				shutdown(cluster)
			})

			leaderCount := 0
			err := internal.RetryGeneric(6, 200*time.Millisecond, func() (*rpc.Client, error) {
				t.Log("trying to get leader")
				leaderCount = 0
				for _, node := range cluster {
					node.mu.Lock()
					if node.state == leaderState {
						leaderCount++
					}
					node.mu.Unlock()
				}
				t.Logf("found %v leader\n", leaderCount)

				if leaderCount != 1 {
					t.Log("retrying again...")
					return nil, fmt.Errorf("leader count expected %v got %v", 1, leaderCount)
				}
				t.Log("done...")

				return nil, nil
			})
			if err != nil {
				t.Fatalf("failed to get correct number of leaders %v", err)
			}

		})
	}

}

func getNodeAddrs(n int) (res []string) {
	for i := 0; i < n; i++ {
		res = append(res, fmt.Sprintf("30%d%d", n, i))
	}

	return
}

func bootstrapCluster(numberOfNodes int) []*Node {
	var nodeCluster []*Node

	for _, nodeAddr := range getNodeAddrs(numberOfNodes) {
		n := NewNode(nodeAddr)
		nodeCluster = append(nodeCluster, n)
	}

	c := Cluster{Nodes: nodeCluster}
	go c.Start()

	return nodeCluster
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

	return nil
}

func TestRemoveNodeById(t *testing.T) {
	tests := map[string]struct {
		nodes []*Node
		id    string
		want  []*Node
	}{
		"Remove existing node": {
			nodes: []*Node{
				{id: "node1"},
				{id: "node2"},
				{id: "node3"},
			},
			id: "node2",
			want: []*Node{
				{id: "node1"},
				{id: "node3"},
			},
		},
		"Remove non-existing node": {
			nodes: []*Node{
				{id: "node1"},
				{id: "node2"},
				{id: "node3"},
			},
			id: "node4",
			want: []*Node{
				{id: "node1"},
				{id: "node2"},
				{id: "node3"},
			},
		},
		"Remove from empty slice": {
			nodes: []*Node{},
			id:    "node1",
			want:  []*Node{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := removeFromNodeCluster(tt.nodes, tt.id)
			if len(got) != len(tt.want) {
				t.Errorf("removeNodeById() = %v, want %v", got, tt.want)
			}
			for i, node := range got {
				if node.id != tt.want[i].id {
					t.Errorf("removeNodeById() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
