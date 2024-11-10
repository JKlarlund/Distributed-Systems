package Node

import (
	"context"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Clock"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"google.golang.org/grpc"
	"log"
	"net"
	"sync"
	"time"
)

type NodeServer struct {
	pb.UnimplementedConsensusServer
	NodeID             int32
	Clock              *Clock.LClock
	RequestedTimestamp int32 // When node requested access to critical section. Should me initialized to MAX INT
	Mutex              sync.Mutex
	Clients            map[string]pb.ConsensusClient // Stores addresses of discovered nodes, excluding itself
	SelfAddress        string                        // Store the current node's address
}

var nodeIsWaitingForAccess = false
var nodeIsInCriticalSection = false

func InitializeDiscovery(node *NodeServer) {
	nodeAddresses := []string{
		"127.0.0.1:5000",
		"127.0.0.1:5001",
		"127.0.0.1:5002",
	}
	for i := 0; i < len(nodeAddresses); i++ {
		if nodeAddresses[i] != node.SelfAddress {
			node.initializeConnection(nodeAddresses[i])

			log.Printf("Node added: %s", nodeAddresses[i])
		}
	}
}

func (s *NodeServer) StartGRPCServer() {
	listener, err := net.Listen("tcp", s.SelfAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", s.SelfAddress, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterConsensusServer(grpcServer, s)

	log.Printf("Node %d is running gRPC server at %s", s.NodeID, s.SelfAddress)

	// Start the server in a goroutine so it runs concurrently
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()
}

// Inserts a k,v pair from our map
func (s *NodeServer) initializeConnection(target string) {
	_, ok := s.Clients[target]
	if !ok {
		conn, err := grpc.Dial(target, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Println(err.Error())
		}
		s.Clients[target] = pb.NewConsensusClient(conn)

	}
}

// Deletes a k,v pair from our map
func (s *NodeServer) severConnection(target string) {
	_, ok := s.Clients[target]
	if ok {
		delete(s.Clients, target)
	}
}

func (s *NodeServer) RequestAccessToCriticalSection(wg1 *sync.WaitGroup) {
	defer wg1.Done()
	nodeIsWaitingForAccess = true

	s.RequestedTimestamp = s.Clock.SendEvent() // Incrementing timestamp once before saving it
	var wg sync.WaitGroup
	wg.Add(len(s.Clients))

	request := &pb.AccessRequest{
		NodeID:    s.NodeID,
		Timestamp: s.RequestedTimestamp,
		Address:   s.SelfAddress,
	}

	//We need to wait for clients to have n-1 approvals. Then go write something.
	for _, client := range s.Clients {
		go s.requestSingleAccess(&client, request, &wg)
	}

	wg.Wait()

	s.emulateCriticalSection()

	nodeIsWaitingForAccess = false

}

func (s *NodeServer) emulateCriticalSection() {
	// Increments the lamport clock for entering the critical section
	s.Clock.Step()
	log.Println(fmt.Sprintf("Node %d is entering the critical section at lamport time %d", s.NodeID, s.Clock.Time))

	nodeIsInCriticalSection = true
	// Simulates doing something in the critical section
	time.Sleep(2 * time.Second)

	// Increments the lamport clock for leaving the critical section
	s.Clock.Step()
	log.Println(fmt.Sprintf("Node %d is leaving the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	nodeIsInCriticalSection = false
}

func (s *NodeServer) requestSingleAccess(client *pb.ConsensusClient, request *pb.AccessRequest, wg *sync.WaitGroup) {
	defer wg.Done()

	context, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.Clock.SendEvent() // Incrementing the lamport clock for each send event
	_, err := (*client).RequestAccess(context, request)
	if err != nil {
		log.Printf("Error in Request Single Access to %s: %v", request.Address, err)
	}
}

// RequestAccess The purpose is to receive a request, decide if access should be granted to the requesting node, and
// send a response back to the requesting node.
func (s *NodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Updated and incremented local clock since this is a receive event
	s.Clock.ReceiveEvent(req.Timestamp)

	log.Printf("Node %d received access request from Node %d with timestamp %d\n", s.NodeID, req.NodeID, req.Timestamp)

	// Checking if access should be granted to the requesting node
	accessGranted := shouldHaveAccess(req.Timestamp, s.RequestedTimestamp)
	response := &pb.AccessResponse{
		NodeID:    s.NodeID,
		Timestamp: s.Clock.SendEvent(), // Incrementing the Lamport clock
		Access:    accessGranted,
	}
	for !accessGranted {
		time.Sleep(time.Second)
		if shouldHaveAccess(req.Timestamp, s.RequestedTimestamp) {
			return response, nil
		}
	}

	return response, nil
}

func shouldHaveAccess(requestTime int32, OwnRequestTime int32) bool {
	// Checking if the node is in the critical section
	if nodeIsInCriticalSection {
		return false
	}
	// Checking if the node requested access before this node
	if nodeIsWaitingForAccess && requestTime > OwnRequestTime {
		return false
	}
	return true
}
