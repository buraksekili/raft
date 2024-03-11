package raft

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/buraksekili/raft/internal"
)

type logEntry struct {
	Term int
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
	listener        net.Listener

	shutDownRPCServer context.CancelFunc
}

// sendRequestVote sends RequestVote RPC to other servers.
// The function is locked by mutex.
func (n *Node) sendRequestVote() {
	initialTerm := n.currentTerm
	lastLogIdx := len(n.log) - 1

	lastLogTerm := 0
	if lastLogIdx > -1 {
		lastLogTerm = n.log[lastLogIdx].Term
	} else if lastLogIdx < 0 {
		lastLogIdx = 0
	}

	if len(n.cluster) == 1 && n.cluster[0] == n.id {
		n.l("i am the only node in the cluster, becoming the leader")
		n.setLeader()
		return
	}

	// initially is 1 since 1 vote coming from the candidate itself
	totalVotes := 1

	for _, serverId := range n.cluster {
		if serverId != n.id {
			// Invoked by candidates to gather votes
			rvr := new(RequestVoteReq)
			rvr.CandidateTerm = initialTerm
			rvr.CandidateId = n.id
			rvr.LastLogIdx = lastLogIdx
			rvr.LastLogTerm = lastLogTerm

			rvres := new(RequestVoteRes)

			go func(sId string, req *RequestVoteReq, res *RequestVoteRes) {
				n.l("sending req to %v", sId)
				if err := n.rpc(sId, requestvoteRpcMethodname, req, res); err != nil {
					n.err("failed to send %v to serverId: %v, err: %v", requestvoteRpcMethodname, sId, err)
				}

				n.mu.Lock()
				n.l("received RequestVoteResponse, %+v", res)
				defer n.mu.Unlock()

				if n.state != candidateState {
					n.l("i am not candidate, returning")
					return
				}

				if res.Term > initialTerm {
					n.l("resp term:%v is greater than mine %v", res.Term, initialTerm)
					n.setFollower(res.Term)
					return
				}

				if res.Term == initialTerm {
					if res.VoteGranted {
						totalVotes++
						n.l("incremented vote, new totalVotes: %v", totalVotes)
						if totalVotes >= (len(n.cluster)+1)/2 {
							n.l("totalVotes: %v, cluster size %v, quorum %v", totalVotes, len(n.cluster), (len(n.cluster)+1)/2)
							n.setLeader()
							return
						}
					}
				}
				return
			}(serverId, rvr, rvres)
		}
	}
}

func (n *Node) sendHeartbeat() {
	n.mu.Lock()
	if n.state != leaderState {
		n.mu.Unlock()
		return
	}

	// sending more RPC call increases memory usage.
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	initialTerm := n.currentTerm
	cluster := n.cluster
	n.mu.Unlock()

	for _, serverId := range cluster {
		if serverId != n.id {
			<-t.C
			go func(id string) {
				n.mu.Lock()
				res := new(AppendEntriesRes)
				req := new(AppendEntriesReq)
				req.Term = n.currentTerm
				req.LeaderID = n.id
				n.mu.Unlock()

				if err := n.rpc(id, appendEntriesRpcMethodname, req, res); err != nil {
					n.err("failed to send %v to serverId: %v, err: %v", appendEntriesRpcMethodname, id, err)
					return
				}

				n.mu.Lock()
				if res.Term > initialTerm {
					n.setFollower(res.Term)
				}
				n.mu.Unlock()
				return
			}(serverId)
		}
	}
}

// if timeout passes, start leader election. otherwise, do not start leader election.
func (n *Node) startLeaderElection() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state == leaderState {
		return
	}

	if time.Since(n.stateStartTime) > n.electionTimeout {
		n.l("election will start, t/o: %v, stateStartTime: %v", n.electionTimeout, n.stateStartTime.Format(time.TimeOnly))
		n.currentTerm++
		n.votedFor = n.id
		n.state = candidateState
		n.stateStartTime = time.Now()
		n.sendRequestVote()

		return
	}
}

// start runs a new goroutine within a new infinite loop where Raft logic runs periodically (based on electionTimeout).
func (n *Node) start() error {
	n.mu.Lock()
	n.cluster = appendIfNotExists(n.cluster, n.id)

	serv := rpc.NewServer()
	serv.Register(n)

	shutDownCtx, cancel := context.WithCancel(context.Background())
	n.shutDownRPCServer = cancel

	var err error
	n.listener, err = net.Listen("tcp", fmt.Sprintf(":%v", n.id))
	if err != nil {
		return err
	}

	stop := make(chan error)
	stopReporter := make(chan error)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func(server *rpc.Server) {
		for {
			conn, err := n.listener.Accept()
			if err != nil {
				select {
				case <-shutDownCtx.Done():
					fmt.Println("shutdown context received")
					wg.Done()
					return
				}
			} else {
				go func() {
					wg.Add(1)
					server.ServeConn(conn)
					wg.Done()
				}()
			}
		}
	}(serv)

	for _, nodeAddr := range n.cluster {
		if nodeAddr != n.id {
			client, err := internal.RetryRPCDial(fmt.Sprintf("127.0.0.1:%v", nodeAddr))
			if err != nil {
				return err
			}

			n.serverClients[nodeAddr] = client
		}
	}

	n.mu.Unlock()

	wg.Add(1)
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		run := true
		for run {
			select {
			case <-t.C:
				n.report()
			case err := <-stopReporter:
				n.l("stopping reporter since the node has stopped, %v", err)
				run = false
				break
			}
		}

		wg.Done()
		return
	}()

	wg.Add(1)
	go func() {
		n.mu.Lock()
		n.running = true
		n.stateStartTime = time.Now()
		n.mu.Unlock()

		for {
			n.mu.Lock()
			currState := n.state
			isRunning := n.running
			if !isRunning {
				stopReporter <- fmt.Errorf("stopping current node %v", n.id)
				n.mu.Unlock()
				break
			}
			n.mu.Unlock()

			switch currState {
			case followerState:
				n.startLeaderElection()
			case candidateState:
				n.startLeaderElection()
			case leaderState:
				n.sendHeartbeat()
			}
		}

		wg.Done()
		stop <- nil
		return
	}()

	wg.Wait()

	return <-stop
}

func (n *Node) report() {
	n.mu.Lock()
	defer n.mu.Unlock()
	msg := fmt.Sprintf("[REPORT] id: %v#%v\tterm: %v",
		n.id, n.state, n.currentTerm,
	)

	log.Printf(msg)
}

func (n *Node) l(msg string, args ...interface{}) {
	msg = fmt.Sprintf("id: %v#%v\tterm %v\tmsg: %v\n",
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
	n.mu.Lock()
	rpcClient := n.serverClients[nodeId]
	n.mu.Unlock()

	if rpcClient == nil {
		return fmt.Errorf("RPC Client not initialized yet")
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
	n.l("becoming a leader")
	n.state = leaderState
	n.stateStartTime = time.Now()
}

func (n *Node) Start() error {
	return n.start()
}

func (n *Node) Stop() {
	n.running = false
}

func (n *Node) processRequestVote(candidateId string, candidateTerm, lastLogIdx, lastLogTerm int) (int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	term, voteGranted := 0, false

	if candidateTerm > n.currentTerm {
		n.state = followerState
		n.currentTerm = candidateTerm
		n.votedFor = ""
		n.stateStartTime = time.Now()
	}

	if candidateTerm < n.currentTerm {
		return n.currentTerm, false
	}

	// If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote.
	//
	// Raft determines which of two logs is more up-to-date by comparing
	// the index and term of the last entries in the logs.
	// - If the logs have last entries with different terms, then the log
	// with the later term is more up-to-date.
	// Check for candidate's last log term `lastLogTerm` >  s.PersistentState.Log[len(s.PersistentState.Log) - 1].Term
	// - If the logs end with the same term, then whichever log is longer is more up-to-date.

	currLogLastIdx := len(n.log) - 1
	currLogLastTerm := -1
	if currLogLastIdx > -1 {
		currLogLastTerm = n.log[currLogLastIdx].Term
	}

	votedForOk := n.votedFor == "" || n.votedFor == candidateId
	logsOk := (lastLogTerm > currLogLastTerm) || (lastLogTerm == currLogLastTerm && lastLogIdx >= currLogLastIdx)
	if votedForOk && logsOk {
		voteGranted = true
		n.votedFor = candidateId
		n.stateStartTime = time.Now()
	}

	term = n.currentTerm

	return term, voteGranted
}

// appendEntries invoked by leader to replicate log entries (§5.3); also used as heartbeat
// RECEIVER IMPLEMENTATION.
func (n *Node) receiveAppendEntries(req *AppendEntriesReq) (term int, success bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	term = n.currentTerm

	if req.Term > n.currentTerm {
		n.l("found a server with greater term, becoming follower")
		n.setFollower(req.Term)
	}

	if req.Term == n.currentTerm && (n.state == candidateState || n.state == followerState) {
		n.state = followerState
	}

	// 1- Reply false if term < CurrentTerm (§5.1)
	if req.Term < n.currentTerm {
		return 0, false
	}

	n.stateStartTime = time.Now()

	// 2- reply false if log doesn’t contain an entry at prevLogIndex
	// whose term matches prevLogTerm
	process := len(n.log) > req.PrevLogIdx && n.log[req.PrevLogIdx].Term == req.PrevLogTerm
	if !process {
		return 0, false
	}

	// 3- If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (§5.3)
	if n.log[req.PrevLogIdx].Term != req.PrevLogTerm {
		return 0, false
	}

	return 0, false
}

func (n *Node) shutdown() {
	n.l("shutting down...")
	n.running = false
	n.state = followerState
	n.currentTerm = 0
}

func (n *Node) CloseServer() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.running = false

	fmt.Println("closing server => ", n.id)
	if n.listener != nil {
		err := n.listener.Close()
		if err != nil {
			return err
		}
		n.listener = nil
	}

	n.shutDownRPCServer()

	for _, id := range n.cluster {
		fmt.Println("updating => ", id)
		if n.serverClients[id] != nil {
			if err := n.serverClients[id].Close(); err == nil {
				n.serverClients[id] = nil
			}
		}

	}

	return nil
}

func remove(s []string, item string) []string {
	for i, v := range s {
		if v == item {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
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

func NewNode(id string, cluster []string) *Node {
	return &Node{
		id:              id,
		cluster:         cluster,
		serverClients:   make(map[string]*rpc.Client),
		electionTimeout: time.Duration(rand.Intn(400)+600) * time.Millisecond,
		state:           followerState,
	}
}

func getRandomDuration(intervalMin, intervalMax int) time.Duration {
	return time.Duration(rand.Intn(intervalMin)+intervalMax) * time.Millisecond
}
