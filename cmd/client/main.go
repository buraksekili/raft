package main

import (
	"flag"
	"fmt"
	"log"
	"net/rpc"

	"github.com/buraksekili/raft"
)

var f = flag.String("addr", "3000", "")

func main() {
	flag.Parse()

	client, err := rpc.Dial("tcp", fmt.Sprintf("localhost:%s", *f))
	if err != nil {
		log.Fatal("dialing:", err)
	}

	// Call the Node.Close method
	r := new(raft.RequestVoteReq)
	s := new(raft.RequestVoteRes)
	err = client.Call("Node.Close", r, s)
	if err != nil {
		log.Fatal("calling:", err)
	}
}
