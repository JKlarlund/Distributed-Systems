package Node

import (
	"context"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Clock"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	discovery "github.com/fuhrmannb/node-discovery"
	"google.golang.org/grpc"
	"log"
	"net/url"
	"os"
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
	selfAddress        string                        // Store the current node's address
}

var nodeIsWaitingForAccess = false
var nodeIsInCriticalSection = false

func InitializeDiscovery(node *NodeServer) {
	//Using node-discovery package.
	myNodeDiscovery, _ := discovery.Listen() //Listens to a default address, we don't have to insert anything.
	hostname, _ := os.Hostname()
	serviceURL, _ := url.Parse("http://" + hostname + ":" + fmt.Sprint(node.NodeID))
	myNodeDiscovery.Register(serviceURL)
	nodeEventChannel := make(chan discovery.NodeEvent)
	myNodeDiscovery.Subscribe(nodeEventChannel)
	go node.handleDiscoveryEvent(&nodeEventChannel)
}

func (s *NodeServer) handleDiscoveryEvent(channel *chan discovery.NodeEvent) {
	log.Println("Før")
	for event := range *channel {
		log.Println("Efter")
		nodeAddress := event.Service.Host
		if event.Type == discovery.ServiceJoinEvent {
			//Service has joined the cluster
			if nodeAddress != s.selfAddress { // Exclude itself
				s.initializeConnection(nodeAddress)
				log.Printf("Node joined: %s", nodeAddress)
			}
		}
		if event.Type == discovery.ServiceLeaveEvent {
			//Service has left the cluser.
			s.severConnection(nodeAddress)
			log.Printf("Node left: %s", nodeAddress)
		}
	}
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

func (s *NodeServer) RequestAccessToCriticalSection() {
	nodeIsWaitingForAccess = true

	s.RequestedTimestamp = s.Clock.SendEvent() // Incrementing timestamp once before saving it
	var wg sync.WaitGroup
	wg.Add(len(s.Clients))

	request := &pb.AccessRequest{
		NodeID:    s.NodeID,
		Timestamp: s.RequestedTimestamp,
		Address:   s.selfAddress,
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
	nodeIsInCriticalSection = true
	log.Println(fmt.Sprintf("Node %d is in the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	time.Sleep(2 * time.Second)
	log.Println(fmt.Sprintf("Node %d is leaving the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	nodeIsInCriticalSection = false
}

func (s *NodeServer) requestSingleAccess(client *pb.ConsensusClient, request *pb.AccessRequest, wg *sync.WaitGroup) {
	defer wg.Done()

	context, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.Clock.SendEvent() // Incrementing the lamport clock for each send event
	_, err := (*client).RequestAccess(context, request)
	if err != nil {
		log.Fatalf("Something went wrong in Request Single Access")
	}
}

// RequestAccess The purpose is to receive a request, decide if access should be granted to the requesting node, and
// send a response back to the requesting node.
func (s *NodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	log.Printf("Received access request from Node %d with timestamp %d\n", req.NodeID, req.Timestamp)

	// Updated and incremented local clock since this is a receive event
	s.Clock.ReceiveEvent(req.Timestamp)

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
