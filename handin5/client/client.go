package client

import (
	"bufio"
	"context"
	"fmt"
	Clock "github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"log"
	"os"
	"strconv"
	"strings"
)

type Client struct {
	Clock *Clock.LClock
	ID    int32
}

var clientInstance Client

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
	clientInstance = Client{ID: response.UserID, Clock: Clock.InitializeLClock(2)}

	go listenToStream(stream)
	go readInput(client)

	select {}
}

func listenToStream(stream pb.AuctionService_AuctionStreamClient) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("Error receiving message from stream: %v", err)
			return
		}
		log.Printf("Server: %v", msg)
	}
}

func readInput(client pb.AuctionServiceClient) {
	for {
		reader := bufio.NewReader(os.Stdin)
		message, err := reader.ReadString('\n')
		bidInt, err := strconv.Atoi(strings.TrimSpace(message))
		if err != nil {
			fmt.Println("\033[1;31mBid not sent since the input is not a valid integer\u001B[0m")
			continue
		}
		bid := int32(bidInt)
		clientInstance.Clock.SendEvent()
		log.Printf("Sending a ")
		_, err = client.Bid(context.Background(), &pb.BidRequest{Amount: bid, BidderId: clientInstance.ID})
		if err != nil {
			log.Printf("Error sending bid: %v", err)
		}
	}
}
