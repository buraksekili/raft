# RAFT

## Leader Election

### Known Bugs
- When there are two nodes in the cluster and if one node is closed explicitly (`Node.Close`), 
the other node gets stuck in an infinite loop in the candidate state.

## Log Replication

## Safety
