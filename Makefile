RAFT_REPLICACOUNT ?= 1
RAFT_ENABLEPROFILING ?= false
run:
	go build -o bin/raft ./cmd && ./bin/raft --replica=${RAFT_REPLICACOUNT} --profiling=${RAFT_ENABLEPROFILING}
