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
	"time"
)

type Client struct {
	Clock *Clock.LClock
	ID    int32
	log   *log.Logger
	timer time.Time
}

var clientInstance Client
var client pb.AuctionServiceClient

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	logFile := logs.InitLoggerWithUniqueID("Client")
	if err != nil {
		log.Fatalf("User failed connecting to auction: %v", err)
		logs.WriteToLog(logFile, fmt.Sprintf("User failed connecting to auction: %v", err), 0, -1) //No user ID yet.
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Trying to connect to the auction at lamport: %v", 0)
	logs.WriteToLog(logFile, fmt.Sprintf("Trying to connect to the auction"), 0, -1)

	client = pb.NewAuctionServiceClient(conn)

	response, err := client.Join(context.Background(), &pb.JoinRequest{Timestamp: 1})
	if err != nil {
		log.Printf("User failed to join the AuctionStream")
		logs.WriteToLog(logFile, fmt.Sprintf("Trying to connect to the auction"), response.Timestamp, response.UserID)
	}
	clientInstance = Client{ID: response.UserID, Clock: Clock.InitializeLClock(2), log: logFile}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)

	stream, err := client.AuctionStream(context.Background())
	if err == nil {
		log.Printf("Connection was established as user: %d at lamport: %d", clientInstance.ID, clientInstance.Clock.Time)
		logs.WriteToLog(clientInstance.log, fmt.Sprintf("Connection was established as user: %d", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)

	}
	err = stream.Send(&pb.AuctionMessage{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.SendEvent(),
		Message:   "Initial connection message",
	})
	if err != nil {
		log.Printf("Error sending initial message: %v", err)
		logs.WriteToLog(clientInstance.log, fmt.Sprintf("Error sending initial message: %v", err), clientInstance.Clock.Time, clientInstance.ID)

	}

	go listenToStream(stream)
	go readInput(client)

	<-sigs

	LeaveResponse, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: clientInstance.ID, Timestamp: clientInstance.Clock.SendEvent()})
	if err != nil {
		log.Printf("User: %d failed to leave the AuctionStream", clientInstance.ID)
		logs.WriteToLog(clientInstance.log, fmt.Sprintf("User: %d failed to leave the AuctionStream", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)

	}
	clientInstance.Clock.ReceiveEvent(LeaveResponse.Timestamp)
	stream.CloseSend()
	log.Printf("User: %d successfully left the auction!", clientInstance.ID)
	logs.WriteToLog(clientInstance.log, fmt.Sprintf("User: %d successfully left the auction!", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)

}

func listenToStream(stream pb.AuctionService_AuctionStreamClient) {
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("Server closed the stream.")
				logs.WriteToLog(clientInstance.log, fmt.Sprintf("User: %d successfully left the auction!", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)

				return
			}
			log.Printf("Error while receiving message: %v", err)
			logs.WriteToLog(clientInstance.log, fmt.Sprintf("Error while receiving message: %v", err), clientInstance.Clock.Time, clientInstance.ID)

			return
		}

		// Process the incoming message
		if in != nil {
			clientInstance.Clock.ReceiveEvent(in.Timestamp)
			log.Printf("%v", in.Message)
			logs.WriteToLog(clientInstance.log, "Message received from stream", clientInstance.Clock.Time, clientInstance.ID)

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
			log.Printf("Sending a bid to the server at lamport time: %d", clientInstance.Clock.Time)
			logs.WriteToLog(clientInstance.log, "Sending a bid to server", clientInstance.Clock.Time, clientInstance.ID)

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
	logs.WriteToLog(clientInstance.log, fmt.Sprintf("Requesting current result from server"), clientInstance.Clock.Time, clientInstance.ID)

	if err != nil {
		log.Printf("Getting the result caused an error: %v", err)
		logs.WriteToLog(clientInstance.log, fmt.Sprintf("Getting the result caused an error: %v", err), clientInstance.Clock.Time, clientInstance.ID)

		return
	}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)
	logs.WriteToLog(clientInstance.log, fmt.Sprintf("Received current result from server"), clientInstance.Clock.Time, clientInstance.ID)

	if response.AuctionEnded {
		log.Printf("Highest bid was: %d by user: %d", response.HighestBid, response.HighestBidder)
	} else {
		log.Printf("Current highest bid is: %d by user: %d", response.HighestBid, response.HighestBidder)
	}
}

func CheckLeaderAlive() bool {
	for {
		time.Sleep(1 * time.Second)

		// Create a context with a 40-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()

		// Perform the RPC call with the context
		startTime := time.Now()
		responseCh := make(chan bool) // Channel to signal if the call succeeds

		go func() {
			_, err := client.Heartbeat(ctx, &pb.HeartbeatMessage{
				Timestamp: clientInstance.Clock.SendEvent(),
			})
			if err != nil {
				fmt.Printf("Heartbeat failed: %v\n", err)
				responseCh <- false
				return
			}
			responseCh <- true
		}()

		select {
		case success := <-responseCh:
			if !success {
				return false // If the RPC call failed
			}
			elapsed := time.Since(startTime)
			fmt.Printf("Heartbeat succeeded, response time: %v\n", elapsed)
			clientInstance.timer = time.Now() // Update the timer
		case <-ctx.Done():
			// Context timeout occurred (40 seconds exceeded)
			fmt.Println("Heartbeat timed out: No response from the leader.")
			return false
		}
	}
}
