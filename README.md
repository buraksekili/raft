# RAFT

## Leader Election

Implemented leader election based on the RAFT algorithm. Simple test scenarios can be found in the test file.

## Log Replication
Not yet started; will work on it after leader election.

## Safety
Not yet started; will work on it after log replication.

## Run
### Sample server
```bash
make run RAFT_ENABLEPROFILING=false RAFT_REPLICACOUNT=3
```
Runs a cluster of 3 nodes.

### Sample client to remove a node
```bash
go run cmd/client/main.go -addr 3000
```
Removes the node that listens on 127.0.0.1:3000.

## Restrictions
At the moment, there is no host discovery implemented. So, the program runs assuming it's running on localhost."