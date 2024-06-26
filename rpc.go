package raft

type RequestVoteReq struct {
	CandidateTerm int
	CandidateId   string
	LastLogIdx    int
	LastLogTerm   int
}
type RequestVoteRes struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesReq struct {
	// Leader's term
	Term        int        `json:"term,omitempty"`
	LeaderID    string     `json:"leaderID,omitempty"`
	PrevLogIdx  int        `json:"prevLogIdx,omitempty"`
	PrevLogTerm int        `json:"prevLogTerm,omitempty"`
	Entries     []logEntry `json:"entries,omitempty"`
	// leader's commitIndex
	LeaderCommit int `json:"leaderCommit,omitempty"`
}
type AppendEntriesRes struct {
	// CurrentTerm, for leader to update itself
	Term int

	Success bool
}

const (
	requestvoteRpcMethodname   = "Node.RequestVote"
	appendEntriesRpcMethodname = "Node.AppendEntries"
)

func (n *Node) RequestVote(req *RequestVoteReq, reply *RequestVoteRes) error {
	currentTerm, voteGranted := n.handleRequestVote(req.CandidateId, req.CandidateTerm, req.LastLogIdx, req.LastLogTerm)

	reply.Term = currentTerm
	reply.VoteGranted = voteGranted

	return nil
}

func (n *Node) AppendEntries(req *AppendEntriesReq, reply *AppendEntriesRes) error {
	if reply == nil {
		reply = new(AppendEntriesRes)
	}

	reply.Term, reply.Success = n.handleAppendEntriesRequest(req)

	return nil
}

type CmdReq struct {
	Add   bool
	Cmd   string
	Count int
}

type CmdRes struct {
	Res bool
}

func (n *Node) Add(req *CmdReq, reply *CmdRes) error {
	if req.Add {
		n.log = append(n.log, logEntry{Term: n.currentTerm, Command: req.Cmd})
	} else {
		if req.Count == 0 {
			req.Count = 1
		}
	}

	reply.Res = true

	return nil
}
