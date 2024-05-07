/*
reference: https://github.com/storj/minio/blob/e70e32900cff68bbf12c7aa06b8c8d72dd2879d3/pkg/dsync/rpc-client-impl_test.go
*/
package raft

import (
	"net/rpc"
	"sync"
)

type RpcClientWithRetry struct {
	mutex sync.Mutex
	rpc   *rpc.Client
	addr  string
}

func rpcClientWithRetry(addr string) *RpcClientWithRetry {
	return &RpcClientWithRetry{addr: addr}
}

// Close closes the underlying socket file descriptor.
func (rpcClient *RpcClientWithRetry) Close() error {
	rpcClient.mutex.Lock()
	defer rpcClient.mutex.Unlock()
	// If rpc client has not connected yet there is nothing to close.
	if rpcClient.rpc == nil {
		return nil
	}

	// Reset rpcClient.rpc to allow for subsequent calls to use a new
	// (socket) connection.
	clnt := rpcClient.rpc
	rpcClient.rpc = nil
	return clnt.Close()
}

// Call makes a RPC call to the remote endpoint using the default codec, namely encoding/gob.
func (rpcClient *RpcClientWithRetry) Call(serviceMethod string, args interface{}, reply interface{}) (err error) {
	rpcClient.mutex.Lock()
	defer rpcClient.mutex.Unlock()
	dialCall := func() error {
		// If the rpc.Client is nil, we attempt to (re)connect with the remote endpoint.
		if rpcClient.rpc == nil {
			clnt, derr := rpc.Dial("tcp", rpcClient.addr)
			if derr != nil {
				return derr
			}
			rpcClient.rpc = clnt
		}
		// If the RPC fails due to a network-related error, then we reset
		// rpc.Client for a subsequent reconnect.
		return rpcClient.rpc.Call(serviceMethod, args, reply)
	}
	if err = dialCall(); err == rpc.ErrShutdown {
		rpcClient.rpc.Close()
		rpcClient.rpc = nil
		err = dialCall()
	}
	return err
}

func (rpcClient *RpcClientWithRetry) String() string {
	return "http://" + rpcClient.addr
}
