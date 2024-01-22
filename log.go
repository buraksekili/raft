package raft

import "fmt"

type logger struct {
	server *Server
}

func log(s *Server, msg string, args ...interface{}) {
	msg = fmt.Sprintf("[LOG]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.ID, s.Raft.State, s.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}

func (s *logger) log(msg string, args ...interface{}) {
	msg = fmt.Sprintf("[LOG]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.server.ID, s.server.Raft.State, s.server.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}

func (s *logger) err(msg string, args ...interface{}) {
	msg = fmt.Sprintf("[ERROR]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.server.ID, s.server.Raft.State, s.server.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}
