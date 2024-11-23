package main

import (
	"bufio"
	"context"
	"fmt"
	Clock "github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5/logs"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"io"
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
	log   *log.Logger
}

var clientInstance Client

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	log := logs.InitLogger("Client")
	if err != nil {
		logs.WriteToLog(log, "User failed to connect to auction", 0, -1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	logs.WriteToLog(log, "Trying to connect to the auction", 0, -1)

	client := pb.NewAuctionServiceClient(conn)

	response, err := client.Join(context.Background(), &pb.JoinRequest{Timestamp: 1})
	if err != nil {
		log.Printf("User failed to join the AuctionStream")
	}
	clientInstance = Client{
		ID:    response.UserID,
		Clock: Clock.InitializeLClock(2),
		log:   log,
	}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)

	stream, err := client.AuctionStream(context.Background())
	if err == nil {
		logs.WriteToLog(log, "Connection was established", clientInstance.Clock.Time, clientInstance.ID)
	}
	err = stream.Send(&pb.AuctionMessage{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.SendEvent(),
		Message:   "Initial connection message",
	})
	if err != nil {
		logs.WriteToLog(log, "Error sending initial message", clientInstance.Clock.Time, clientInstance.ID)
	}

	go listenToStream(stream)
	go readInput(client)

	<-sigs

	LeaveResponse, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: clientInstance.ID, Timestamp: clientInstance.Clock.SendEvent()})
	if err != nil {
		logs.WriteToLog(clientInstance.log, "Failed to leave the AuctionStream", clientInstance.Clock.Time, clientInstance.ID)
	}
	clientInstance.Clock.ReceiveEvent(LeaveResponse.Timestamp)
	stream.CloseSend()
	logs.WriteToLog(clientInstance.log, "User successfully left the auction!", clientInstance.Clock.Time, clientInstance.ID)

}

func listenToStream(stream pb.AuctionService_AuctionStreamClient) {
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("Server closed the stream.")
				logs.WriteToLog(clientInstance.log, "Server closed the stream.", clientInstance.Clock.Time, clientInstance.ID)

				return
			}
			logs.WriteToLog(clientInstance.log, "Error while receiving message", clientInstance.Clock.Time, clientInstance.ID)

			return
		}

		// Process the incoming message
		if in != nil {
			clientInstance.Clock.ReceiveEvent(in.Timestamp)
			log.Printf("%v", in.Message)
		}
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
				log.Printf("Bid not sent since the input is not a valid integer")
				continue
			}
			bid := int32(bidInt)
			response, err := client.Bid(context.Background(), &pb.BidRequest{Amount: bid, BidderId: clientInstance.ID, Timestamp: clientInstance.Clock.SendEvent()})
			logs.WriteToLog(clientInstance.log, "Sending a bid to the server", clientInstance.Clock.Time, clientInstance.ID)

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
			logs.WriteToLog(clientInstance.log, fmt.Sprintf("Your bid of: %d was accepted!", bidInt), clientInstance.Clock.Time, clientInstance.ID)

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
	logs.WriteToLog(clientInstance.log, "Getting result", clientInstance.Clock.Time, clientInstance.ID)

	if response.AuctionEnded {
		log.Printf("Highest bid was: %d by user: %d", response.HighestBid, response.HighestBidder)

	} else {
		log.Printf("Current highest bid is: %d by user: %d", response.HighestBid, response.HighestBidder)

	}
}
