# RAFT




## Leader Election

Implemented leader election based on the RAFT algorithm. Simple test scenarios can be found in the test file.

## Log Replication
Simple log replication added.

## Safety
Not yet started; will work on it after log replication.

## Run
### Sample server
```bash
make run RAFT_ENABLEPROFILING=false RAFT_REPLICACOUNT=3
```
Runs a cluster of 3 nodes.

![raft-demo1080p](https://github.com/buraksekili/raft/assets/32663655/62f8b59f-a35d-479e-a4f0-bd432f45592b)

## Restrictions
At the moment, there is no host discovery implemented. So, the program runs assuming it's running on localhost.
