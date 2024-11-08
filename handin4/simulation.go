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
	var wg sync.WaitGroup
	wg.Add(nodesCount)

	ports := []int32{5000, 5001, 5002}

	for i := 0; i < nodesCount; i++ {
		go initializeNode(ports[i], &wg)
		time.Sleep(time.Second)
	}
	wg.Wait()
	log.Println("DONE!")
}

func initializeNode(port int32, wg *sync.WaitGroup) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	node := Node.NodeServer{
		NodeID:             port,
		Clock:              Clock.InitializeLClock(0),
		RequestedTimestamp: math.MaxInt32,
		Clients:            make(map[string]pb.ConsensusClient),
		Mutex:              sync.Mutex{},
		SelfAddress:        address,
	}
	node.StartGRPCServer()

	log.Printf("Node %d initialized at %s", node.NodeID, node.SelfAddress)

	Node.InitializeDiscovery(&node)

	// Waiting for a random duration of time before requesting access to critical section
	time.Sleep(time.Second * 3)
	time.Sleep(time.Duration(rand.Intn(10)+1) * time.Second)
	node.RequestAccessToCriticalSection(wg)
}
