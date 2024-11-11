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
	NodeID                 int32
	Queue                  Queue
	Clock                  *Clock.LClock
	RequestedTimestamp     int32 // When node requested access to critical section. Should me initialized to MAX INT
	Mutex                  sync.Mutex
	Clients                map[string]pb.ConsensusClient // Stores addresses of discovered nodes, excluding itself
	SelfAddress            string                        // Store the current node's address
	NodeIsWaitingForAccess bool
}

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
		s.Clients[target] = pb.NewConsensusClient(conn)

	}
}

func (s *NodeServer) LeaveCriticalSection() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	release := &pb.ReleaseRequest{
		NodeID:    s.NodeID,
		Timestamp: s.Clock.SendEvent(),
	}

	// Broadcast a release message to all nodes (without expecting a response)
	for _, client := range s.Clients {
		go func(client pb.ConsensusClient) {

			_, err := (client).Release(context.Background(), release)
			if err != nil {
				log.Printf("Error broadcasting release from Node %d: %v", s.NodeID, err)
			}
		}(client)
	}
	// Removes itself from the queue
	s.Queue.Remove(s.NodeID)
	s.NodeIsWaitingForAccess = false
}

func (s *NodeServer) Release(ctx context.Context, req *pb.ReleaseRequest) (*pb.ReleaseResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Receive event
	s.Clock.ReceiveEvent(req.Timestamp)
	// Remove the request from the local priority queue
	for _, request := range s.Queue {
		if request.NodeID == req.NodeID {
			s.Queue.Remove(req.NodeID)
			break
		}
	}
	// Log the release reception
	log.Printf("Node %d received release from Node %d at Lamport time %d\n", s.NodeID, req.NodeID, s.Clock.Time)

	// Return a response (optional)
	return &pb.ReleaseResponse{Ack: true}, nil
}

func (s *NodeServer) RequestAccessToCriticalSection(mainWG *sync.WaitGroup) {

	if s.NodeIsWaitingForAccess {
		time.Sleep(2 * time.Second)
		s.tryAccessCriticalSection(mainWG)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(s.Clients))

	request := &pb.AccessRequest{
		NodeID:    s.NodeID,
		Timestamp: s.Clock.SendEvent(), // Incrementing timestamp once before saving it
		Address:   s.SelfAddress,
	}
	s.addToPriorityQueue(request.Timestamp, s.NodeID)
	//We need to wait for clients to have n-1 approvals. Then go write something.
	for _, client := range s.Clients {
		go s.requestSingleAccess(&client, request, &wg)
	}
	s.NodeIsWaitingForAccess = true
	wg.Wait()
	mainWG.Done()
}

func (s *NodeServer) tryAccessCriticalSection(mainWG *sync.WaitGroup) {
	if len(s.Queue) > 0 && s.Queue.GetLowest() == s.NodeID {
		s.emulateCriticalSection(mainWG) // Method to handle entry into the critical section
		return
	}
	mainWG.Done()
}

func (s *NodeServer) emulateCriticalSection(mainWG *sync.WaitGroup) {
	defer mainWG.Done()
	// Increments the lamport clock for entering the critical section
	s.Clock.Step()
	log.Println(fmt.Sprintf("Node %d is entering the critical section at lamport time %d", s.NodeID, s.Clock.Time))

	// Simulates doing something in the critical section
	time.Sleep(2 * time.Second)

	// Increments the lamport clock for leaving the critical section
	s.Clock.Step()
	log.Println(fmt.Sprintf("Node %d has left the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	// Broadcasting to all clients that the client has left the critical section
	s.LeaveCriticalSection()
}

func (s *NodeServer) requestSingleAccess(client *pb.ConsensusClient, request *pb.AccessRequest, wg *sync.WaitGroup) {
	defer wg.Done()

	context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	message, err := (*client).RequestAccess(context, request)
	if err != nil {
		log.Printf("Error in Request Single Access to %s: %v", request.Address, err)
	}
	s.Clock.ReceiveEvent(message.Timestamp)

}

func (s *NodeServer) addToPriorityQueue(timestamp int32, nodeID int32) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Queue.Add(timestamp, nodeID)
}

// RequestAccess The purpose is to receive a request, decide if access should be granted to the requesting node, and
// send a response back to the requesting node.
func (s *NodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {

	// Updated and incremented local clock since this is a receive event
	s.Clock.ReceiveEvent(req.Timestamp)

	log.Printf("Node %d received access request from Node %d with timestamp %d\n", s.NodeID, req.NodeID, req.Timestamp)

	s.addToPriorityQueue(req.Timestamp, req.NodeID)

	response := &pb.AccessResponse{
		NodeID:    s.NodeID,
		Timestamp: s.Clock.SendEvent(), // Incrementing the Lamport clock
	}

	return response, nil
}
