RAFT_REPLICA ?= 1
run:
	go build -o bin/raft ./cmd && ./bin/raft --replica ${RAFT_REPLICA}
