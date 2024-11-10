package main

import (
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Clock"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Node"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"math"
	"sync"
	"time"
)

func main() {
	nodesCount := 3
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < nodesCount; i++ {
		fmt.Println(i)
		go initializeNode(int32(i), &wg)
		time.Sleep(time.Second)
	}
	wg.Wait()
}

func initializeNode(id int32, wg *sync.WaitGroup) {
	defer wg.Done()
	node := Node.NodeServer{
		NodeID:             id,
		Clock:              Clock.InitializeLClock(0),
		RequestedTimestamp: math.MaxInt32,
		Clients:            make(map[string]pb.ConsensusClient),
		Mutex:              sync.Mutex{},
	}
	Node.InitializeDiscovery(&node)

	// Waiting for a random duration of time before requesting access to critical section
	//time.Sleep(time.Second * 3)
	//time.Sleep(time.Duration(rand.Intn(10)+1) * time.Second)
	for {
		if len(node.Clients) == 2 {
			fmt.Println("Length is 2 now")
			break
		}
	}
	node.RequestAccessToCriticalSection()
}
