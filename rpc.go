package raft

import "fmt"

type RequestVoteReq struct {
	CandidateTerm int
	CandidateId   string
	LastLogIdx    int
	LastLogTerm   int
}
type RequestVoteRes struct {
	Res         string
	Term        int
	VoteGranted bool
}

type AppendEntriesReq struct {
	// Leader's term
	Term        int        `json:"term,omitempty"`
	LeaderID    string     `json:"leaderID,omitempty"`
	PrevLogIdx  int        `json:"prevLogIdx,omitempty"`
	PrevLogTerm int        `json:"prevLogTerm,omitempty"`
	Entries     []LogEntry `json:"entries,omitempty"`
	// leader's commitIndex
	LeaderCommit int `json:"leaderCommit,omitempty"`
}
type AppendEntriesRes struct {
	// CurrentTerm, for leader to update itself
	Term int

	Success bool
}

type RaftRPC struct {
	server *Server
}

const (
	requestvoteRpcMethodname   = "Server.RequestVote"
	appendentriesRpcMethodname = "Server.AppendEntries"
	addNewServerRpcMethodName  = "Server.AddNewServer"
)

// RequestVote sends RPC request
func (s *Server) RequestVote(req *RequestVoteReq, reply *RequestVoteRes) error {
	currentTerm, voteGranted := s.requestVote(req.CandidateTerm, req.CandidateId, req.LastLogIdx, req.LastLogTerm)
	reply.Term = currentTerm
	reply.VoteGranted = voteGranted
	return nil
}

func (s *Server) AppendEntries(req *AppendEntriesReq, reply *AppendEntriesRes) error {
	if reply == nil {
		reply = new(AppendEntriesRes)
	}

	s.appendEntries(req)

	return nil
}

type AddNewServerReq struct {
	BaseServerID string
	NewServerID  string
}

type AddNewServerRes struct {
	Added   bool
	Started bool
}

func (s *Server) AddNewServer(req *AddNewServerReq, reply *AddNewServerRes) error {
	fmt.Println("add new server called")
	newServer := NewServer(req.NewServerID, []string{})

	err := s.addNewServer(newServer)
	if err == nil {
		reply.Added = true
	}

	newServer.ServerIds = s.ServerIds

	if s.Raft.State == leader {
		err := newServer.Start()
		if err != nil {
			s.L.err("failed to start new server, err: %v", err)
			reply.Started = false
		}

		reply.Started = true
	}
	return nil
}
