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
