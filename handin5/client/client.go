package main

import (
	"bufio"
	"context"
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

	response, err := client.Join(context.Background(), &pb.JoinRequest{Timestamp: 1})
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
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}
		message = strings.TrimSpace(message)
		parts := strings.SplitN(message, " ", 2)
		command := strings.ToLower(parts[0])

		switch command {
		case "bid":
			if len(parts) < 2 {
				log.Printf("Invalid bid command. Usage: bid <amount>")
				continue
			}
			bidInt, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Printf("\033[1;31mBid not sent since the input is not a valid integer\u001B[0m")
				continue
			}
			bid := int32(bidInt)
			response, err := client.Bid(context.Background(), &pb.BidRequest{Amount: bid, BidderId: clientInstance.ID, Timestamp: clientInstance.Clock.SendEvent()})
			log.Printf("Sending a bid to the server at lamport time: %d", clientInstance.Clock.Time)
			if err != nil {
				log.Printf("Error sending bid: %v", err)
				continue
			}
			clientInstance.Clock.ReceiveEvent(response.Timestamp)
			if !response.Success {
				log.Printf("Bid is too low!")
				continue
			}
			log.Printf("Your bid of: %d was accepted! at lamport: %d", bidInt, clientInstance.Clock.Time)
		case "result":
			getResult(client)
		case "help":
			log.Printf("result - To get the highest bid")
			log.Printf("bid <amount> - To bid on the auction")
		default:
			log.Printf("Unknow command: %v, use 'help' to get a list of the commands", command)
		}
	}
}

func getResult(client pb.AuctionServiceClient) {
	response, err := client.Result(context.Background(), &pb.ResultRequest{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.SendEvent(),
	})
	if err != nil {
		log.Printf("Getting the result caused an error: %v", err)
		return
	}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)
	if response.AuctionEnded {
		log.Printf("Highest bid was: %d by user: %d", response.HighestBid, response.HighestBidder)
	} else {
		log.Printf("Current highest bid is: %d by user: %d", response.HighestBid, response.HighestBidder)
	}
}
