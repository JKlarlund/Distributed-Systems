package Node

import (
	"google.golang.org/grpc"
	"log"
	"net"
	"rafttest/Clock"
	"rafttest/rpc"
	"sync"
)

type NodeServer struct {
	rpc.UnimplementedRaftServer
	NodeID             int32
	Clock              *Clock.LClock
	RequestedTimestamp int32 // When node requested access to critical section. Should me initialized to MAX INT
	Mutex              sync.Mutex
	Clients            map[string]rpc.RaftClient // Stores addresses of discovered nodes, excluding itself
	SelfAddress        string                    // Store the current node's address
}

func (s *NodeServer) StartGRPCServer() {
	listener, err := net.Listen("tcp", s.SelfAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", s.SelfAddress, err)
	}

	grpcServer := grpc.NewServer()
	rpc.RegisterRaftServer(grpcServer, s)

	log.Printf("Node %d is running gRPC server at %s", s.NodeID, s.SelfAddress)

	// Start the server in a goroutine so it runs concurrently
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in gRPC server for Node %d: %v", s.NodeID, r)
			}
		}()
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Node %d gRPC server stopped serving: %v", s.NodeID, err)
		}
	}()
}

// Inserts a k,v pair from our map
func (s *NodeServer) initializeConnection(target string) {
	_, ok := s.Clients[target]
	if !ok {
		conn, err := grpc.Dial(target, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Printf("Failed to connect to %s: %v", target, err)
			return // Prevent adding a nil client
		}
		s.Clients[target] = rpc.NewRaftClient(conn)

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

			log.Printf("Node added: %s", nodeAddresses[i])
		}
	}
}
