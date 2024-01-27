package main

import (
	"fmt"

	"github.com/buraksekili/raft"
)

func main() {
	n := raft.NewNode("1234", []string{})
	fmt.Println("creating a new node")
	if err := n.Start(); err != nil {
		panic(err)
	}
}
