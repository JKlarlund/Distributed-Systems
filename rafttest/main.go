package main

import (
	"fmt"
	"log"
	"rafttest/Clock"
	Node "rafttest/Node"
	"rafttest/rpc"

	"sync"
)

func main() {
	basePort := 5000
	for i := 1; i < 5; i++ {
		go initializeNode(*intToInt32(basePort + i))
	}
	initializeNode(*intToInt32(basePort))
}

func initializeNode(port int32) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	node := Node.NodeServer{
		NodeID:      port,
		Clock:       Clock.InitializeLClock(0),
		Clients:     make(map[string]rpc.RaftClient),
		Mutex:       sync.Mutex{},
		SelfAddress: address,
	}
	node.StartGRPCServer()

	log.Printf("Node %d initialized at %s", node.NodeID, node.SelfAddress)

	Node.InitializeDiscovery(&node)

}

func intToInt32(i int) *int32 {
	value := int32(i)
	return &value
}
