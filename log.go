package raft

import "fmt"

type Logger struct {
	Server *Server
}

func log(s *Server, msg string, args ...interface{}) {
	msg = fmt.Sprintf("[LOG]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.ID, s.Raft.State, s.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}

func (s *Logger) log(msg string, args ...interface{}) {
	msg = fmt.Sprintf("[LOG]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.Server.ID, s.Server.Raft.State, s.Server.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}

func (s *Logger) err(msg string, args ...interface{}) {
	msg = fmt.Sprintf("[ERROR]\tserverId: %v\tserverState: %v\tcurrentTerm: %v\tmsg: %v\n",
		s.Server.ID, s.Server.Raft.State, s.Server.Raft.PersistentState.CurrentTerm, msg)
	fmt.Printf(msg, args...)
}
