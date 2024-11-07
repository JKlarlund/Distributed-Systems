package handin4

import (
	"context"
	"fmt"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	discovery "github.com/fuhrmannb/node-discovery"
	"google.golang.org/grpc"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"time"
)

type nodeServer struct {
	pb.UnimplementedConsensusServer
	nodeID             int32
	clock              *LClock
	requestedTimestamp int32 // When node requested access to critical section. Should me initialized to MAX INT
	replyQueue         []*pb.AccessRequest
	mutex              sync.Mutex
}

var nodeIsWaitingForAccess = false
var nodeIsInCriticalSection = false

func main(logger *log.Logger, id int32) {
	node := nodeServer{
		nodeID:             id,
		clock:              InitializeLClock(0),
		requestedTimestamp: math.MaxInt32,
		clients:            make(map[string]pb.ConsensusClient),
		mutex:              sync.Mutex{},
	}

	initializeDiscovery(&node)

	// Waiting for a random duration of time before requesting access to critical section
	time.Sleep(time.Duration(rand.Intn(10)+1) * time.Second)
	node.requestAccessToCriticalSection()
}

func initializeDiscovery(node *nodeServer) {
	//Using node-discovery package.
	myNodeDiscovery, _ := discovery.Listen() //Listens to a default address, we don't have to insert anything.
	hostname, _ := os.Hostname()
	serviceURL, _ := url.Parse("http://" + hostname + ":" + fmt.Sprint(node.nodeID))
	myNodeDiscovery.Register(serviceURL)
	nodeEventChannel := make(chan discovery.NodeEvent)
	myNodeDiscovery.Subscribe(nodeEventChannel)
	go node.handleDiscoveryEvent(&nodeEventChannel)
}

func (s *nodeServer) handleDiscoveryEvent(channel *chan discovery.NodeEvent) {
	for event := range *channel {
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
func (s *nodeServer) initializeConnection(target string) {
	_, ok := s.clients[target]
	if !ok {
		conn, _ := grpc.Dial(target, grpc.WithInsecure(), grpc.WithBlock())
		s.clients[target] = pb.NewConsensusClient(conn)
	}
}

// Deletes a k,v pair from our map
func (s *nodeServer) severConnection(target string) {
	_, ok := s.clients[target]
	if ok {
		delete(s.clients, target)
	}
}

func (s *nodeServer) requestAccessToCriticalSection() {
	nodeIsWaitingForAccess = true

	s.requestedTimestamp = s.clock.SendEvent() // Incrementing timestamp once before saving it
	var wg sync.WaitGroup
	wg.Add(len(s.clients))

	request := &pb.AccessRequest{
		NodeID:    s.nodeID,
		Timestamp: s.requestedTimestamp,
		Address:   s.selfAddress,
	}

	//We need to wait for clients to have n-1 approvals. Then go write something.
	for _, client := range s.clients {
		go s.requestSingleAccess(&client, request, &wg)
	}

	wg.Wait()

	s.emulateCriticalSection()

	nodeIsWaitingForAccess = false

}

// RequestAccess The purpose is to receive a request, decide if access should be granted to the requesting node, and
// send a response back to the requesting node.
func (s *nodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	fmt.Printf("Received access request from Node %d with timestamp %d\n", req.NodeID, req.Timestamp)

	// Updated and incremented local clock since this is a receive event
	s.clock.ReceiveEvent(req.Timestamp)

	// Checking if access should be granted to the requesting node
	accessGranted := shouldHaveAccess(req.Timestamp, s.requestedTimestamp)

	response := &pb.AccessResponse{
		NodeID:    s.nodeID,
		Timestamp: s.clock.SendEvent(), // Incrementing the Lamport clock
		Access:    accessGranted,
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
