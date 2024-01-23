package main

import (
	"flag"
	"fmt"
	"log"
	"net/rpc"

	"github.com/buraksekili/raft"
)

var newNodeId = flag.String("node", "", "")
var requestServerId = flag.String("request", "3000", "")

func main() {
	flag.Parse()

	if *newNodeId == "" {
		log.Fatal("invalid node id")
	}

	if *requestServerId == "" {
		log.Fatal("invalid node id")
	}

	fmt.Println("dummy client => ", *requestServerId)

	c, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%v", *requestServerId))
	if err != nil {
		panic(err)
	}
	defer c.Close()

	req := raft.AddNewServerReq{
		NewServerID:  *newNodeId,
		BaseServerID: *requestServerId,
	}
	res := new(raft.AddNewServerRes)

	if err = c.Call("Server.AddNewServer", &req, res); err != nil {
		panic(err)
	}

	fmt.Println("received => ", res.Added)

}
