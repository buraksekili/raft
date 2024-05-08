# RAFT

## Leader Election

Implemented leader election based on the RAFT algorithm. Simple test scenarios can be found in the test file.

## Log Replication
(wip) Simple log replication added.

## Safety
(wip)

## Run

Runs 3 nodes:

```bash
go run ./cmd/server/main.go --id 0
go run ./cmd/server/main.go --id 1
go run ./cmd/server/main.go --id 2
```

## Restrictions
At the moment, there is no host discovery implemented. So, the program runs assuming it's running on localhost.
