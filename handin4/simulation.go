package main

import (
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Clock"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Node"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

func main() {
	nodesCount := 3

	ports := []int32{5000, 5001, 5002}

	for i := 0; i < nodesCount; i++ {
		go initializeNode(ports[i])
		time.Sleep(2 * time.Second)
	}

	// Keep the main function running indefinitely
	select {}
}

func initializeNode(port int32) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	node := Node.NodeServer{
		NodeID:                 port,
		Clock:                  Clock.InitializeLClock(0),
		RequestedTimestamp:     math.MaxInt32,
		Clients:                make(map[string]pb.ConsensusClient),
		Mutex:                  sync.Mutex{},
		SelfAddress:            address,
		NodeIsWaitingForAccess: false,
		Queue:                  Node.Queue{},
	}
	node.Queue.NewQueue()
	node.StartGRPCServer()

	log.Printf("Node %d initialized at %s", node.NodeID, node.SelfAddress)

	Node.InitializeDiscovery(&node)

	// Waiting for a random duration of time before requesting access to critical section
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
	for {
		var mainWG = sync.WaitGroup{}
		mainWG.Add(1)
		node.RequestAccessToCriticalSection(&mainWG)
		mainWG.Wait()
		time.Sleep(5 * time.Second)
	}

}
