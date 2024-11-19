package client

import (
	"context"
	Clock "github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"log"
)

type Client struct {
	Clock *Clock.LClock
	ID    int32
}

func (c Client) Bid(ctx context.Context, in *pb.BidRequest, opts ...grpc.CallOption) (*pb.BidResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) Result(ctx context.Context, in *pb.ResultRequest, opts ...grpc.CallOption) (*pb.ResultResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) AuctionStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[pb.AuctionMessage, pb.AuctionMessage], error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) Join(ctx context.Context, in *pb.JoinRequest, opts ...grpc.CallOption) (*pb.JoinResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c Client) Leave(ctx context.Context, in *pb.LeaveRequest, opts ...grpc.CallOption) (*pb.LeaveResponse, error) {
	//TODO implement me
	panic("implement me")
}

var client Client

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User failed connecting to auction: %v", err)
	}
	log.Printf("Trying to connect to the auction at lamport: %v", 0)

	client := pb.NewAuctionServiceClient(conn)

	response, err := client.Join(context.Background(), &pb.JoinRequest{})
	if err != nil {
		log.Printf("User failed to join the AuctionStream")
	}

	stream, err := client.AuctionStream(context.Background())
	if err == nil {
		log.Printf("Connection was established as user: %d", response.UserID)

	}
	client = Client{ID: response.UserID, Clock: Clock.InitializeLClock(2)}

}
