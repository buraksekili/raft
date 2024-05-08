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
	Term    int
	Command interface{}
}

type Node struct {
	mu        sync.RWMutex
	id        string
	state     raftState
	cluster   []string
	nodes     []*Node
	totalVote int
	running   bool

	/*
		PERSISTENT STATE ON ALL SERVERS
	*/
	currentTerm int
	votedFor    string
	log         []logEntry // first index is 1 (in the paper).
	persistence Persistence

	/*
		VOLATILE STATE ON ALL SERVERS
	*/
	commitIndex int
	lastApplied int

	/*
		VOLATILE STATE ON LEADERS
	*/
	// nextIndex is the index of the next log entry the leader will send to that follower specified in the key.
	// (initialized to leader last log index + 1). When a leader first comes to power, it
	// initializes all nextIndex values to the index just after the last one in its log.
	// it is like the length of follower's log?
	// liderin gonderecegi logdan sonraki gelen log indeksi
	nextIndex map[string]int
	// matchIndex is that for each server, index of highest log entry known to be replicated on server
	matchIndex map[string]int

	// stateStartTime corresponds to time of when this Node starts
	// functioning as state.
	stateStartTime  time.Time
	electionTimeout time.Duration
	serverClients   map[string]*RpcClientWithRetry
	listener        net.Listener

	shutDownRPCServer context.CancelFunc
	logger            internal.Logger
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

	if len(n.nodes) == 1 && n.nodes[0].id == n.id {
		n.setLeader()
		return
	}

	// initially is 1 since 1 vote coming from the candidate itself
	totalVotes := 1

	for _, node := range n.nodes {
		if node.id != n.id {
			// Invoked by candidates to gather votes
			rvr := new(RequestVoteReq)
			rvr.CandidateTerm = initialTerm
			rvr.CandidateId = n.id
			rvr.LastLogIdx = lastLogIdx
			rvr.LastLogTerm = lastLogTerm

			rvres := new(RequestVoteRes)

			go func(sId string, req *RequestVoteReq, res *RequestVoteRes) {
				n.info("Sending RequestVoteReq", "request", *req, "peer", sId)
				if err := n.rpc(sId, requestvoteRpcMethodname, req, res); err != nil {
					return
				}

				n.mu.Lock()
				defer n.mu.Unlock()
				n.info("Received RequestVoteResponse", "response", *res)

				if n.state != candidateState {
					n.info("i am not candidate anymore", "myState", n.state)
					return
				}

				// if the response sent by any of node includes a greater term, then fall back to follower.
				// assume that your cluster contains 3 nodes at term 1. If one of the follower crashes and restarts
				// again, it might send a RequestVoteRPC before receiving an AppendEntry RPC.
				// As the follower's initialTerm is 1 (as the term is incremented before starting RequestVote RPCs)
				// while the leader's term is also 1, you might need to consider what happens
				// sent and the current leader's term is also 1.
				if res.Term > initialTerm {
					n.info("=> becoming follower")
					n.setFollower(res.Term)
					return
				} else if res.Term == initialTerm {
					if res.VoteGranted {
						totalVotes++
						if totalVotes >= (len(n.nodes)+1)/2 {
							n.setLeader()
							return
						}
					}
				}
				return
			}(node.id, rvr, rvres)
		}
	}
}

func (n *Node) prevLogIndex(id string) int {
	return n.nextIndex[id] - 1
}

func (n *Node) prevLogTerm(id string) int {
	i := n.prevLogIndex(id)
	if i == -1 || len(n.log) == 0 {
		return 0
	}

	return n.log[i].Term
}

func (n *Node) sendHeartbeat() {
	n.mu.Lock()
	if n.state != leaderState {
		n.mu.Unlock()
		return
	}

	t := time.NewTicker(25 * time.Millisecond)
	defer t.Stop()
	initialTerm := n.currentTerm
	nodes := n.nodes
	n.mu.Unlock()

	for _, node := range nodes {
		if node.id != n.id {
			go func(id string) {
				n.mu.Lock()
				res := new(AppendEntriesRes)

				prevLogIndex := n.prevLogIndex(id)
				prevLogTerm := n.prevLogTerm(id)

				nextIdxForFollower := n.nextIndex[id]
				lastLogIdx := len(n.log)

				suffix := []logEntry{}
				if lastLogIdx >= nextIdxForFollower {
					suffix = n.log[nextIdxForFollower:]
				}

				lastNextIdx := n.nextIndex[id]
				req := &AppendEntriesReq{
					PrevLogIdx:   prevLogIndex,
					Entries:      suffix,
					Term:         n.currentTerm,
					PrevLogTerm:  prevLogTerm,
					LeaderCommit: n.commitIndex,
					LeaderID:     n.id,
				}

				if len(suffix) > 0 {
					n.info("Replicating log", "receiver_id", id, "suffix", suffix)
				}
				n.mu.Unlock()

				if err := n.rpc(id, appendEntriesRpcMethodname, req, res); err != nil {
					return
				}

				n.mu.Lock()
				defer n.mu.Unlock()
				// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
				if res.Term > initialTerm {
					n.setFollower(res.Term)
					return
				}

				if res.Term != req.Term && n.state == leaderState {
					return
				}

				if res.Success {
					if req.PrevLogIdx+len(suffix) >= nextIdxForFollower {
						// add + 1 bc prevLogIndex might be -1 when the log is empty.
						n.nextIndex[id] = prevLogIndex + len(suffix) + 1
					}

				} else {
					// If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
					n.nextIndex[id] = max(lastNextIdx-1, 0)
				}

				return
			}(node.id)

			<-t.C
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
		n.info("election will start")
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
	serv := rpc.NewServer()
	serv.Register(n)

	storage, err := NewBoltDBStorage(fmt.Sprintf("node-%v.db", n.id))
	if err != nil {
		return err
	}
	n.persistence = storage

	n.logger = internal.NewLogger(nil)
	n.info("Initializing the server...")

	if n.persistence.HasData() {
		n.restoreFromStorage()
		n.logger.Info("we already have info", "afterTerm", n.currentTerm)
	}

	n.initializeNextIndex()

	if n.matchIndex == nil {
		n.matchIndex = make(map[string]int)
	}

	shutDownCtx, cancel := context.WithCancel(context.Background())
	n.shutDownRPCServer = cancel

	n.listener, err = net.Listen("tcp", fmt.Sprintf(":%v", n.id))
	if err != nil {
		return err
	}

	stop := make(chan error)
	stopReporter := make(chan error)

	// we can embed wg to node, so that easily keep track of running goroutines for Node
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func(server *rpc.Server) {
		for {
			conn, err := n.listener.Accept()
			if err != nil {
				select {
				case <-shutDownCtx.Done():
					fmt.Println("shutdown context received in ", n.id)
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

	for _, node := range n.nodes {
		if node.id != n.id {
			n.serverClients[node.id] = rpcClientWithRetry(fmt.Sprintf("127.0.0.1:%v", node.id))
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
				n.err("stopping reporter since the node has stopped", err)
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
		//counter := 0
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

		n.running = false
		wg.Done()
		stop <- nil
		return
	}()

	wg.Wait()

	return <-stop
}

func (n *Node) getaddrs() []string {
	var a []string
	for _, v := range n.nodes {
		a = append(a, v.id)
	}

	return a
}

func (n *Node) report() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.logger.Report("", "id", n.id, "term", n.currentTerm, "state", n.state, "log", fmt.Sprintf("%+v", n.log))
}

func (n *Node) info(msg string, args ...interface{}) {
	n.logger.Info(msg, append(args, "term", n.currentTerm, "id", n.id, "state", n.state)...)
}

func (n *Node) err(msg string, err error, args ...interface{}) {
	keys := ""
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]

		keys = fmt.Sprintf("%v %v=%v ", keys, key, value)
	}

	log.Printf("ERROR %v %verror=%s", msg, keys, err)
}

func (n *Node) rpc(nodeId, rpcMethod string, args, reply interface{}) error {
	n.mu.Lock()
	rpcClient := n.serverClients[nodeId]
	n.mu.Unlock()

	return rpcClient.Call(rpcMethod, args, reply)
}

func (n *Node) setFollower(newTerm int) {
	n.state = followerState
	n.currentTerm = newTerm
	n.votedFor = ""
	n.stateStartTime = time.Now()
}

func (n *Node) setLeader() {
	n.state = leaderState
	for _, node := range n.nodes {
		if node.id != n.id {
			n.nextIndex[node.id] = len(n.log)
			n.matchIndex[node.id] = 0
		}
	}

	n.stateStartTime = time.Now()
	n.info("became the leader")
}

func (n *Node) Start() error {
	return n.start()
}

func (n *Node) Stop() {
	n.running = false
}

func (n *Node) handleRequestVote(candidateId string, candidateTerm, lastLogIdx, lastLogTerm int) (int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.info("Received RequestVote RPC", "candidateId", candidateId)

	term, voteGranted := 0, false

	if candidateTerm > n.currentTerm {
		n.state = followerState
		n.currentTerm = candidateTerm
		n.votedFor = ""
		n.stateStartTime = time.Now()
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

	n.persist()
	term = n.currentTerm

	return term, voteGranted
}

// appendEntries invoked by leader to replicate log entries (§5.3); also used as heartbeat
func (n *Node) handleAppendEntriesRequest(req *AppendEntriesReq) (term int, success bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	term = n.currentTerm

	if n.state == candidateState || n.state == leaderState {
		n.setFollower(req.Term)
	}

	// 1- Reply false if term < CurrentTerm (§5.1)
	if req.Term < n.currentTerm {
		return n.currentTerm, false
	}

	// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if req.Term > n.currentTerm {
		n.setFollower(req.Term)
	}

	// reset election timeout
	n.stateStartTime = time.Now()

	// 2- reply false if log doesn’t contain an entry at prevLogIndex
	// whose term matches prevLogTerm
	logOk := (req.PrevLogIdx == -1) ||
		(req.PrevLogIdx > -1 && len(n.log) > req.PrevLogIdx && n.log[req.PrevLogIdx].Term == req.PrevLogTerm)
	if !logOk {
		return n.currentTerm, false
	}

	for i, entry := range req.Entries {
		idx := req.PrevLogIdx + i + 1
		if idx < len(n.log) {
			n.log = n.log[:idx]
		}
		n.log = append(n.log, entry)
	}

	n.persist()

	return n.currentTerm, true
}

func (n *Node) initializeNextIndex() {
	if n.nextIndex == nil {
		n.nextIndex = make(map[string]int)
	}
}

func removeFromNodeCluster(nodes []*Node, id string) []*Node {
	var result []*Node
	for _, node := range nodes {
		if node.id != id {
			result = append(result, node)
		}
	}

	return result
}

func (n *Node) SetCluster(cluster []*Node) {
	n.nodes = cluster
}

func NewNode(id string) *Node {
	return &Node{
		id:              id,
		serverClients:   make(map[string]*RpcClientWithRetry),
		electionTimeout: time.Duration(rand.Intn(400)+600) * time.Millisecond,
		state:           followerState,
	}
}
