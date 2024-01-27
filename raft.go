package raft

import (
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"sync"
	"time"
)

type raftState string
type logEntry struct {
	term int
}

const (
	followerState  raftState = "follower"
	candidateState raftState = "candidate"
	leaderState    raftState = "leader"
)

func NewNode(id string, cluster []string) *Node {
	return &Node{
		id:              id,
		cluster:         cluster,
		serverClients:   make(map[string]*rpc.Client),
		electionTimeout: 1 * time.Second,
		state:           followerState,
	}
}

type Node struct {
	mu          sync.RWMutex
	id          string
	currentTerm int
	state       raftState
	votedFor    string
	cluster     []string
	totalVote   int
	running     bool
	log         []logEntry

	// stateStartTime corresponds to time of when this Node starts
	// functioning as state.
	stateStartTime  time.Time
	electionTimeout time.Duration
	serverClients   map[string]*rpc.Client
}

// sendRequestVote sends RequestVote RPC to other servers.
// The function is locked by mutex.
func (n *Node) sendRequestVote() {
	initialTerm := n.currentTerm
	lastLogIdx := len(n.log) - 1

	lastLogTerm := 0
	if lastLogIdx > -1 {
		lastLogTerm = n.log[lastLogIdx].term
	} else if lastLogIdx < 0 {
		lastLogIdx = 0
	}

	// Invoked by candidates to gather votes
	req := new(RequestVoteReq)
	req.CandidateTerm = initialTerm
	req.CandidateId = n.id
	req.LastLogIdx = lastLogIdx
	req.LastLogTerm = lastLogTerm

	res := new(RequestVoteRes)

	// initially is 1 since 1 vote coming from the candidate itself
	totalVotes := 1

	for _, serverId := range n.cluster {
		if serverId != n.id {
			if err := n.rpc(serverId, requestvoteRpcMethodname, req, res); err != nil {
				n.err("failed to send %v to serverId: %v, err: %v", requestvoteRpcMethodname, serverId, err)
				return
			}

			if n.state != candidateState {
				return
			}

			if res.Term > initialTerm {
				n.setFollower(res.Term)
				return
			}

			if res.Term == initialTerm {
				if res.VoteGranted {
					totalVotes++
				}
			}
		}
	}

	if totalVotes >= (len(n.cluster)+1)/2 {
		n.setLeader()
		return
	}
}

func (n *Node) sendHeartbeat() {
	n.l("sending heartbeat")
}

// if timeout passes, start leader election. otherwise, do not start leader election.
func (n *Node) startLeaderElection() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if time.Since(n.stateStartTime) > n.electionTimeout {
		n.l("election will start, cluster size: %v", len(n.cluster))
		n.currentTerm++
		n.votedFor = n.id
		n.stateStartTime = time.Now()
		n.sendRequestVote()
	}
}

// start runs a new goroutine within a new infinite loop where Raft logic runs periodically (based on electionTimeout).
func (n *Node) start() error {
	n.cluster = appendIfNotExists(n.cluster, n.id)
	c := make(chan error)

	go func() {
		n.mu.Lock()
		n.running = true
		n.mu.Unlock()

		t := time.NewTicker(time.Duration(rand.Intn(800)+600) * time.Millisecond)
		defer t.Stop()
		for {
			n.mu.Lock()
			n.stateStartTime = time.Now()
			<-t.C
			s := n.state
			if !n.running {
				n.mu.Unlock()
				c <- fmt.Errorf("node [%v] is not running", n.id)
			}
			n.mu.Unlock()

			switch s {
			case followerState:
				n.l("i am follower")
				n.startLeaderElection()
			case candidateState:
				n.startLeaderElection()
			case leaderState:
				n.l("i am leader")
				n.sendHeartbeat()
			}
		}
	}()

	return <-c
}

func (n *Node) Start() error {
	return n.start()
}

func (n *Node) l(msg string, args ...interface{}) {
	msg = fmt.Sprintf("id: #%v\tstate: %v\tterm: %v\tmsg: %v\n",
		n.id, n.state, n.currentTerm, msg,
	)

	log.Printf(msg, args...)
}

func (n *Node) err(msg string, args ...interface{}) {
	msg = fmt.Sprintf("[ERROR] id: #%v\tstate: %v\tterm: %v\tmsg: %v\n",
		n.id, n.state, n.currentTerm, msg,
	)

	log.Printf(msg, args...)
}

func (n *Node) rpc(nodeId, rpcMethod string, args, reply interface{}) error {
	rpcClient, ok := n.serverClients[nodeId]
	if !ok {
		var err error
		rpcClient, err = rpc.DialHTTP("tcp", fmt.Sprintf("127.0.0.1:%v", nodeId))
		if err != nil {
			return err
		}

		n.serverClients[nodeId] = rpcClient
	}

	return rpcClient.Call(rpcMethod, args, reply)
}

func (n *Node) setFollower(newTerm int) {
	n.state = followerState
	n.currentTerm = newTerm
	n.votedFor = ""
	n.stateStartTime = time.Now()
}

func (n *Node) setLeader() {
	n.l("WON THE ELECTION")
	n.state = leaderState
	n.stateStartTime = time.Now()
}

func appendIfNotExists(servers []string, newServerId string) []string {
	if newServerId == "" {
		return servers
	}

	exists := false
	for _, server := range servers {
		if server == "" {
			continue
		}

		if newServerId == server {
			exists = true
		}
	}

	if !exists {
		servers = append(servers, newServerId)
	}

	return servers
}
