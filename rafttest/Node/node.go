package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"net"
	"rafttest/Clock"
	"rafttest/rpc"
	"sync"
	"time"
)

type NodeServer struct {
	rpc.UnimplementedRaftServer
	NodeID      int32
	Clock       *Clock.LClock
	Mutex       sync.Mutex
	Clients     map[string]rpc.RaftClient // Stores addresses of discovered nodes, excluding itself
	SelfAddress string                    // Store the current node's address
	Rank        int32
	Leader      string
	Timer       time.Time
}

func main() {

	rankptr := flag.Int("rank", 0, "rank of node")
	flag.Parse()
	rank := *intToInt32(*rankptr)
	port := 5000 + rank
	fmt.Println("Port is" + string(port))
	address := fmt.Sprintf("127.0.0.1:%d", port)
	node := NodeServer{
		NodeID:      port,
		Clock:       Clock.InitializeLClock(0),
		Clients:     make(map[string]rpc.RaftClient),
		Mutex:       sync.Mutex{},
		SelfAddress: address,
		Rank:        rank,
		Leader:      fmt.Sprintf("127.0.0.1:5000"),
		Timer:       time.Now(),
	}

	node.StartGRPCServer()

	log.Printf("Node %d initialized at %s", node.NodeID, node.SelfAddress)

	go InitializeDiscovery(&node)

	if node.SelfAddress != node.Leader {
		fmt.Println("You are follower")
		node.CheckLeaderAlive()
	} else {
		node.pulse()
		fmt.Println("You are leader!")
	}

}

func (node *NodeServer) StartGRPCServer() {
	listener, err := net.Listen("tcp", node.SelfAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", node.SelfAddress, err)
	}

	grpcServer := grpc.NewServer()
	rpc.RegisterRaftServer(grpcServer, node)

	log.Printf("Node %d is running gRPC server at %s", node.NodeID, node.SelfAddress)

	// Start the server in a goroutine so it runs concurrently
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in gRPC server for Node %d: %v", node.NodeID, r)
			}
		}()
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Node %d gRPC server stopped serving: %v", node.NodeID, err)
		}
	}()
}

// Inserts a k,v pair from our map
func (node *NodeServer) initializeConnection(target string) {
	_, ok := node.Clients[target]
	if !ok {
		conn, err := grpc.Dial(target, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Printf("Failed to connect to %s: %v", target, err)
			return // Prevent adding a nil client
		}
		node.Clients[target] = rpc.NewRaftClient(conn)

	}
}

func InitializeDiscovery(node *NodeServer) {
	nodeAddresses := []string{
		"127.0.0.1:5000",
		"127.0.0.1:5001",
		"127.0.0.1:5002",
		"127.0.0.1:5003",
		"127.0.0.1:5004",
	}
	for i := 0; i < len(nodeAddresses); i++ {
		if nodeAddresses[i] != node.SelfAddress {
			node.initializeConnection(nodeAddresses[i])

			log.Printf("Node %d added: %s", node.NodeID, nodeAddresses[i])
		}
	}
}

/*
Receiving heartbeats from leader.
*/
func (node *NodeServer) Heartbeat(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	node.Timer = time.Now()
	log.Printf("Received heartbeat from leader.")
	return empty, nil
}

func (node *NodeServer) CheckLeaderAlive() bool {
	for {
		time.Sleep(1 * time.Second)
		if (time.Now().Sub(node.Timer)).Seconds() > 40 { //If the delay is greater than 40 seconds.
			fmt.Println("No pulse")
			return false
		}
	}
}

func (node *NodeServer) pulse() {
	for {
		fmt.Printf("Pulsing")
		for _, node := range node.Clients {
			node.Heartbeat(context.Background(), &emptypb.Empty{})
		}
		time.Sleep(20 * time.Second)
	}
}

func intToInt32(i int) *int32 {
	value := int32(i)
	return &value
}
