package handin4

import (
	"fmt"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"sync"
	"time"
)

type nodeServer struct {
	pb.UnimplementedConsensusServer
	nodeID     int32
	timestamp  int32
	replyQueue []*pb.AccessRequest
	mutex      sync.Mutex
}

var currentlyRequestingAccess bool = false
var executingCriticalSection bool = false

func main(logger *log.Logger) {
	//Simulate something.

}

func requestCriticalSection(nodeID int32, timestamp int32, targetAddress string) {
	target, error := grpc.Dial(targetAddress, grpc.WithInsecure(), grpc.WithBlock())
	if error != nil {
		log.Fatalf("Node %d could not connect to all other nodes, terminating node.", nodeID)
	}
	defer target.Close()

	client := pb.NewConsensusClient(target)

	request := &pb.AccessRequest{
		NodeID:    nodeID,
		Timestamp: timestamp,
	}

	context, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, error := client.RequestAccess(context, request)

	if response.Access {
		log.Printf("Node %d has access at timestamp %d", nodeID, timestamp)
	} else {
		log.Printf("Node %d is not granted access at timestamp %d", nodeID, timestamp)
	}
}

func (s *nodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {
	fmt.Printf("Received access request from Node %d with timestamp %d\n", req.NodeID, req.Timestamp)
	var accessGranted bool
	if shouldHaveAccess(req.Timestamp, s.timestamp) {
		accessGranted = true
	} else {
		accessGranted = false
	}

	response := &pb.AccessResponse{
		NodeID:    s.nodeID,
		Timestamp: s.timestamp,
		Access:    accessGranted,
	}

	return response, nil
}
func (s *nodeServer) processQueue() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var newQueue []*pb.AccessRequest
	for _, request := range s.replyQueue {

		if shouldHaveAccess(request.Timestamp, s.timestamp) {
			response := &pb.AccessResponse{
				NodeID:    s.nodeID,
				Timestamp: s.timestamp,
				Access:    true,
			}
			go s.sendQueuedResponse(request.NodeID, response)
		} else {
			newQueue = append(newQueue, request)
		}
	}
	s.replyQueue = newQueue
}

func (s *nodeServer) sendQueuedResponse(nodeID int32, response *pb.AccessResponse) {
	var address string = ""
	target, error := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if error != nil {
		log.Fatalf("Node %d could not connect to node %d", s.nodeID, nodeID)
	}
	defer target.Close()

	client := pb.NewConsensusClient(target)
	_, error = client.GrantQueuedAccess(context.Background(), response)

	if error != nil {
		log.Fatalf("Node %d could not send grant access message to node %d", s.nodeID, nodeID)
	}
}

// TROR det er sådan her det skal evalueres. Hvis requesten
func shouldHaveAccess(requestTime int32, OwnRequestTime int32) bool {
	if executingCriticalSection {
		return false
	} else {
		if currentlyRequestingAccess {
			if requestTime < OwnRequestTime {
				return true
			}
			return false
		}
		return true
	}
}
