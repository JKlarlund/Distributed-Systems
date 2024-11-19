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

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User failed connecting to auction: %v", err)
	}
	log.Printf("Trying to connect to the auction at lamport: %v", 0)

	client := pb.NewAuctionServiceClient(conn)

	client.
}
