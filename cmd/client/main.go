package main

import (
	"flag"
	"fmt"
	"log"
	"net/rpc"
	"time"

	"github.com/buraksekili/raft"
)

func main() {
	// Define command-line flags
	add := flag.Bool("add", false, "Add command")
	remove := flag.Int("remove", 1, "Remove command")
	addr := flag.String("url", "3000", "address of leader")
	flag.Parse()

	// Create a new RPC client
	client, err := rpc.Dial("tcp", fmt.Sprintf(":%v", *addr))
	if err != nil {
		log.Fatal("dialing:", err)
	}

	// Call the appropriate RPC method based on the command
	var reply raft.CmdRes
	var req raft.CmdReq
	if *add {
		req.Add = true
		req.Cmd = fmt.Sprintf("add-%v", time.Now().Nanosecond())
		err = client.Call("Node.Add", req, &reply)
		if err != nil {
			log.Fatal("Node.Add error:", err)
		}
	} else {
		req.Count = *remove
		err = client.Call("Node.Add", req, &reply)
		if err != nil {
			log.Fatal("Node.Add error:", err)
		}
	}

	fmt.Printf("Node.Add reply: %+v\n", reply)
}
