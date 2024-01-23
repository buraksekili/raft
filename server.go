package raft

import (
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"
)

type ServerState string

const (
	Follower  ServerState = "follower"
	candidate ServerState = "candidate"
	leader    ServerState = "leader"
)

type Persistent struct {
	// currentTerm corresponds to the latest term server has seen.
	CurrentTerm int
	// votedFor corresponds to the candidate id that received vote in current term.
	VotedFor string
	// Log corresponds to the log entries.
	// Each entry contains command for state machine, and term when entry was received by leader.
	Log []LogEntry
}

type LogEntry struct {
	Command string
	Term    int
}

func (p *Persistent) persist() {
}

// Volatile information that server keeps.
type Volatile struct {
	CommitIndex int
	LastApplied int
	NextIndex   []int
	MatchIndex  []int
}

func (s *Server) ConnectOtherServers() error {
	for _, serverId := range s.ServerIds {
		if s.ID == serverId {
			continue
		}

		client, err := rpc.DialHTTP("tcp", fmt.Sprintf(":%v", serverId))
		if err != nil {
			return err
		}

		s.serverClients[serverId] = client

		log(s, "%v has successfully connected to %v", s.ID, serverId)
	}

	return nil
}

type StateMachine struct{}

// Server is the server architecture described in Raft paper for state machines.
// It contains three main parts; Consensus Module (Raft), Log and State machine (your application).
// It also comes with HTTP Server to allow external clients to connect the Server. Also, it has RPC client
// to talk with other servers.
type Server struct {
	mu sync.RWMutex

	//CurrentTerm int

	// ID corresponds to ID of this server.
	ID string

	// ServerIds corresponds to array of IDs of other servers in the cluster.
	// Used to know RPC server addresses of other servers if needed.
	ServerIds []string

	// serverClients corresponds to the map of string and rpc.Client that includes
	// nodeId as key and rpc.Client that will be used to communicate with that server.
	serverClients map[string]*rpc.Client

	// Raft corresponds to RAFT consensus algorithm
	Raft *Raft

	// StateMachine corresponds to methods of your state machine
	StateMachine StateMachine

	// state corresponds to server states; follower, candidate or leader.
	// A server remains in follower state as long as it receives valid
	// RPCs from a leader or candidate
	//state serverState

	L *Logger
}

// RPC sends RPC request to the server identified with nodeId by using rpc.Client that is
// created while dialing that server.
func (s *Server) RPC(nodeId, rpcMethod string, args, reply interface{}) error {
	rpcClient, ok := s.serverClients[nodeId]
	if !ok {
		var err error
		rpcClient, err = rpc.DialHTTP("tcp", fmt.Sprintf("127.0.0.1:%v", nodeId))
		if err != nil {
			return err
		}

		s.serverClients[nodeId] = rpcClient
	}

	return rpcClient.Call(rpcMethod, args, reply)
}

func NewServer(id string, serverIds []string) *Server {
	s := &Server{
		ID:        id,
		ServerIds: serverIds,
		Raft: &Raft{
			State:      Follower,
			minTimeout: 150,
			maxTimeout: 300,
		},
		serverClients: make(map[string]*rpc.Client),
	}

	l := &Logger{Server: s}
	s.L = l
	s.Raft.L = l
	s.Raft.Server = s

	return s
}

func (s *Server) Start() error {
	serv := rpc.NewServer()
	serv.Register(s)
	oldMux := http.DefaultServeMux
	mux := http.NewServeMux()
	http.DefaultServeMux = mux
	serv.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)
	http.DefaultServeMux = oldMux

	l, err := net.Listen("tcp", fmt.Sprintf(":%v", s.ID))
	if err != nil {
		panic(err)
	}
	go http.Serve(l, mux)

	s.Raft.ResetElectionTimer = time.Now()

	go func() {
		for {
			if s.Raft.State == leader {
				s.Raft.sendHeartbeat()
			} else if s.Raft.State == Follower {
				s.L.log("===> i am follower")
				s.Raft.runElectionTimeout()
			} else if s.Raft.State == candidate {
				s.L.log("===> i am candidate")
				s.Raft.runElectionTimeout()
			}
		}
	}()

	return nil
}

// requestVote is invoked by candidates to gather votes.
// RECEIVER IMPLEMENTATION.
// SYNCHRONOUS OP
// It takes candidate's term, candidate requesting vote, index of candidate's last log entry and
// term of candidate's last long entry as method arguments.
// It returns CurrentTerm for candidate to update itself, and boolean to indicate whether candidate
// received a vote or not.
func (s *Server) requestVote(candidateTerm int, candidateId string, lastLogIdx, lastLogTerm int) (term int, voteGranted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if candidateTerm > s.Raft.PersistentState.CurrentTerm {
		s.Raft.State = Follower
		s.Raft.PersistentState.CurrentTerm = candidateTerm
		s.Raft.PersistentState.VotedFor = ""
		s.Raft.ResetElectionTimer = time.Now()
	}

	if candidateTerm < s.Raft.PersistentState.CurrentTerm {
		return s.Raft.PersistentState.CurrentTerm, false
	}

	log(s, "Received RequestVote RPC in %v from %v which is in %v term", s.ID, candidateId, candidateTerm)

	// If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote.
	//
	// Raft determines which of two logs is more up-to-date by comparing
	// the index and term of the last entries in the logs.
	// - If the logs have last entries with different terms, then the log
	// with the later term is more up-to-date.
	// Check for candidate's last log term `lastLogTerm` >  s.PersistentState.Log[len(s.PersistentState.Log) - 1].Term
	// - If the logs end with the same term, then whichever log is longer is more up-to-date.

	currLogLastIdx := len(s.Raft.PersistentState.Log) - 1
	currLogLastTerm := -1
	if currLogLastIdx > -1 {
		currLogLastTerm = s.Raft.PersistentState.Log[currLogLastIdx].Term
	}

	votedForOk := s.Raft.PersistentState.VotedFor == "" || s.Raft.PersistentState.VotedFor == candidateId
	logsOk := (lastLogTerm > currLogLastTerm) || (lastLogTerm == currLogLastTerm && lastLogIdx >= currLogLastIdx)
	if votedForOk && logsOk {
		voteGranted = true
		s.Raft.PersistentState.VotedFor = candidateId
		s.Raft.ResetElectionTimer = time.Now()
	}

	term = s.Raft.PersistentState.CurrentTerm

	log(s, "Response: %v to RequestVote RPC from %v", voteGranted, candidateId)

	return
}

// appendEntries invoked by leader to replicate log entries (§5.3); also used as heartbeat
// RECEIVER IMPLEMENTATION.
func (s *Server) appendEntries(req *AppendEntriesReq) (term int, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	term = s.Raft.PersistentState.CurrentTerm
	s.L.log("received AppendEntriesRPC")

	if req.Term > s.Raft.PersistentState.CurrentTerm {
		s.L.log("found a server with greater term, becoming follower")
		s.Raft.setFollower(req.Term)
	}

	if req.Term == s.Raft.PersistentState.CurrentTerm && (s.Raft.State == candidate || s.Raft.State == Follower) {
		s.Raft.State = Follower
	}

	// 1- Reply false if term < CurrentTerm (§5.1)
	if req.Term < s.Raft.PersistentState.CurrentTerm {
		return 0, false
	}

	s.Raft.ResetElectionTimer = time.Now()

	// 2- reply false if log doesn’t contain an entry at prevLogIndex
	// whose term matches prevLogTerm
	process := len(s.Raft.PersistentState.Log) > req.PrevLogIdx &&
		s.Raft.PersistentState.Log[req.PrevLogIdx].Term == req.PrevLogTerm
	if !process {
		return 0, false
	}

	// 3- If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (§5.3)
	if s.Raft.PersistentState.Log[req.PrevLogIdx].Term != req.PrevLogTerm {
		return 0, false
	}

	return 0, false
}

func (s *Server) setLeader() {
	s.Raft.State = leader
	log(s, "i am the new leader")
}

func (s *Server) addNewServer(newServer *Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newServer.ID == s.ID {
		s.L.log("server already exists")
		return nil
	}

	s.ServerIds = appendIfNotExists(s.ServerIds, newServer.ID)

	newServersClient, err := rpc.DialHTTP("tcp", fmt.Sprintf(":%v", newServer.ID))
	if err != nil {
		return err
	}
	s.serverClients[newServer.ID] = newServersClient

	for _, serverId := range s.ServerIds {
		if serverId != s.ID {
			req := new(AddNewServerReq)
			res := new(AddNewServerRes)
			req.NewServerID = newServer.ID
			req.BaseServerID = serverId

			if err = s.RPC(serverId, addNewServerRpcMethodName, req, res); err != nil {
				return fmt.Errorf("failed to add the server")
			}
		}
	}

	s.L.log("added %v", newServer.ID)

	return nil
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
