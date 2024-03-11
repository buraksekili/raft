package raft

type raftState string

const (
	followerState  raftState = "follower"
	candidateState raftState = "candidate"
	leaderState    raftState = "leader"
)
