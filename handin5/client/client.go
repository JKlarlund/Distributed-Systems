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
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Trying to connect to the auction at lamport: %v", 0)

	client := pb.NewAuctionServiceClient(conn)

	response, err := client.Join(context.Background(), &pb.JoinRequest{})
	if err != nil {
		log.Printf("User failed to join the AuctionStream")
	}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)

	stream, err := client.AuctionStream(context.Background())
	if err == nil {
		log.Printf("Connection was established as user: %d", response.UserID)

	}
	clientInstance = Client{ID: response.UserID, Clock: Clock.InitializeLClock(2)}

	go listenToStream(stream)
	go readInput(client)

	<-sigs

	LeaveResponse, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: clientInstance.ID})
	if err != nil {
		log.Printf("User: %d failed to leave the AuctionStream", clientInstance.ID)
	}
	clientInstance.Clock.ReceiveEvent(LeaveResponse.Timestamp)
	stream.CloseSend()
	log.Printf("User: %d successfully left the auction!", clientInstance.ID)
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
		log.Printf("Sending a bid to the server at lamport time: %d", clientInstance.Clock.Time)
		_, err = client.Bid(context.Background(), &pb.BidRequest{Amount: bid, BidderId: clientInstance.ID})
		if err != nil {
			log.Printf("Error sending bid: %v", err)
		}
	}
}

func getResult(client pb.AuctionServiceClient) {
	response, err = client.Result(context.Background(), &pb.ResultRequest{})
}
