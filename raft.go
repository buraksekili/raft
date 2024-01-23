package raft

import (
	"math/rand"
	"sync"
	"time"
)

// Raft corresponds to the consensus module shown in
// Raft paper (State machine architecture).
// The fields are needed by consensus algorithm to work.
// The field exposes methods purely for consensus algorithm.
// The communication with other servers (nodes) in the cluster,
// use Server as Raft struct only implements consensus algorithm.
type Raft struct {
	mu sync.RWMutex
	L  *Logger

	// State corresponds to states of the server that this Raft owns
	// The server has three states; follower, candidate or leader.
	State              ServerState
	ResetElectionTimer time.Time

	Server *Server

	// persistentState on all servers.
	// update them on stable storage before responding to RPCs.
	PersistentState Persistent

	// persistentState on servers (including the ones for leaders).
	VolatileState Volatile

	minTimeout int
	maxTimeout int
}

// electionTimeout runs a timer for election timeout and based on that it might start a new election.
// If a follower receives no communication over a period of time called the election timeout,
// then it assumes there is no viable leader and begins an election to choose a new leader.
func (r *Raft) runElectionTimeout() {
	r.mu.RLock()
	electionTimeout := randomTimeout(r.minTimeout, r.maxTimeout)
	r.L.log("timeout: %v", electionTimeout)
	r.mu.RUnlock()
	//r.L.log("resetElectionTimer: %v", r.ResetElectionTimer)

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		<-t.C
		//r.L.log("LEADER IS ALIVE")

		elapsed := time.Since(r.ResetElectionTimer)
		//r.L.log("elapsed %v\tt/o %v", elapsed.Seconds(), electionTimeout)
		if elapsed > electionTimeout {
			log(r.Server, "Failed to get heart beat from leader, timeout elapsed, starting election")

			//	If election timeout elapses without receiving AppendEntries
			//	RPC from current leader or granting vote to candidate:
			//	convert to candidate.
			r.State = candidate
			//r.Server.CurrentTerm++
			r.startElection()
			return
		}
	}

}

func (r *Raft) startElection() {
	// • On conversion to candidate, start election:
	//	• Increment CurrentTerm
	//	• Vote for self
	//	• Reset election timer
	//	• Send RequestVote RPCs to all other servers
	log(r.Server, "election will start, cluster size: %v", len(r.Server.ServerIds))

	r.PersistentState.CurrentTerm++
	r.PersistentState.VotedFor = r.Server.ID
	r.ResetElectionTimer = time.Now()

	r.sendRequestVoteRPC()
}

func (r *Raft) sendRequestVoteRPC() {
	initialTerm := r.Server.Raft.PersistentState.CurrentTerm
	lastLogIdx := len(r.PersistentState.Log) - 1

	lastLogTerm := 0
	if lastLogIdx > -1 {
		lastLogTerm = r.PersistentState.Log[lastLogIdx].Term
	} else if lastLogIdx < 0 {
		lastLogIdx = 0
	}

	// Invoked by candidates to gather votes
	req := new(RequestVoteReq)
	req.CandidateTerm = initialTerm
	req.CandidateId = r.Server.ID
	req.LastLogIdx = lastLogIdx
	req.LastLogTerm = lastLogTerm

	res := new(RequestVoteRes)

	// initially is 1 since 1 vote coming from the candidate itself
	totalVotes := 1

	for _, serverId := range r.Server.ServerIds {
		if serverId != r.Server.ID {
			if err := r.Server.RPC(serverId, requestvoteRpcMethodname, req, &res); err != nil {
				r.L.err("failed to send %v to serverId: %v, err: %v", requestvoteRpcMethodname, serverId, err)
				return
			}

			if r.State != candidate {
				return
			}

			if res.Term > initialTerm {
				r.setFollower(res.Term)
				return
			}

			if res.Term == initialTerm {
				if res.VoteGranted {
					totalVotes++
				}
			}
		}
	}

	if totalVotes >= (len(r.Server.ServerIds)+1)/2 {
		log(r.Server, "WON THE ELECTION")
		r.Server.setLeader()
		return
	}

}

func (r *Raft) setFollower(newTerm int) {
	r.State = Follower
	r.Server.Raft.PersistentState.CurrentTerm = newTerm
	r.PersistentState.VotedFor = ""
	r.ResetElectionTimer = time.Now()
}

// sendHeartbeat sends empty AppendEntries RPC call to all other servers
func (r *Raft) sendHeartbeat() {
	r.mu.Lock()
	lockedTerm := r.Server.Raft.PersistentState.CurrentTerm
	r.mu.Unlock()

	for _, serverId := range r.Server.ServerIds {
		if serverId != r.Server.ID {
			time.Sleep(100 * time.Millisecond)
			req := AppendEntriesReq{
				Term:     lockedTerm,
				LeaderID: r.Server.ID,
			}
			res := AppendEntriesRes{}

			if err := r.Server.RPC(serverId, appendentriesRpcMethodname, &req, &res); err != nil {
				r.L.err("failed to send %v to serverId: %v, err: %v", appendentriesRpcMethodname, serverId, err)
				return
			}

			if res.Term > lockedTerm {
				r.setFollower(res.Term)
			}

		}
	}
}

// randomTimeout returns random time out between min and max in ms.
func randomTimeout(min, max int) time.Duration {
	randNumber := rand.Intn(min) + (max - min)
	//return time.Duration(randNumber) * time.Millisecond
	to := randNumber % 4
	if to == 0 {
		to += 1
	}

	return time.Duration(to) * time.Millisecond * 1000
}
