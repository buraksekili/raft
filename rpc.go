package raft

import (
	"encoding/gob"
)

func init() {
	r := new(RequestVoteReq)
	rr := new(RequestVoteRes)
	gob.Register(r)
	gob.Register(rr)
}

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
