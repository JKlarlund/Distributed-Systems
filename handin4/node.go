package handin4

import (
	context2 "context"
	"fmt"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

type nodeServer struct {
	pb.UnimplementedConsensusServer
	nodeID    int32
	timestamp int32
}

func main() {
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

	context, cancel := context2.WithTimeout(context2.Background(), time.Second)
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
	if req.Timestamp < s.timestamp {
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
