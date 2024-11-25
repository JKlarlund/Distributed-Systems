package main

import (
	"bufio"
	"context"
	Clock "github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
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
}

var clientInstance Client
var client pb.AuctionServiceClient

func main() {
	knownAddresses := []string{"localhost:1337", "localhost:1338"} // List of known servers
	primaryAddress := findPrimary(knownAddresses)                  // Find the current primary server

	conn, err := grpc.DialContext(context.Background(), primaryAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User failed connecting to auction: %v", err)
	}
	defer conn.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Trying to connect to the auction at lamport: %v", 0)

	client = pb.NewAuctionServiceClient(conn)

	response, err := client.Join(context.Background(), &pb.JoinRequest{Timestamp: 1})
	if err != nil {
		log.Fatalf("User failed to join the AuctionStream: %v", err)
	}
	clientInstance = Client{ID: response.UserID, Clock: Clock.InitializeLClock(2)}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)

	stream, err := client.AuctionStream(context.Background())
	if err != nil {
		log.Fatalf("Failed to open AuctionStream: %v", err)
	}

	err = stream.Send(&pb.AuctionMessage{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.SendEvent(),
		Message:   "Initial connection message",
	})
	if err != nil {
		log.Printf("Error sending initial message: %v", err)
	}

	go listenToStream(stream, knownAddresses)
	go readInput()

	<-sigs

	LeaveResponse, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: clientInstance.ID, Timestamp: clientInstance.Clock.SendEvent()})
	if err != nil {
		log.Printf("User: %d failed to leave the AuctionStream", clientInstance.ID)
	}
	clientInstance.Clock.ReceiveEvent(LeaveResponse.Timestamp)
	stream.CloseSend()
	log.Printf("User: %d successfully left the auction!", clientInstance.ID)
}

func listenToStream(stream pb.AuctionService_AuctionStreamClient, knownAddresses []string) {
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("Server closed the stream.")
				return
			}
			log.Printf("Error while receiving message: %v", err)

			// Handle reconnection logic
			log.Printf("Attempting to reconnect to the auction...")
			time.Sleep(6 * time.Second) // Temp solution, as the backup server needs to assign itself as primary before we can make the call
			newPrimary := findPrimary(knownAddresses)
			conn, err := grpc.DialContext(context.Background(), newPrimary, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				log.Fatalf("Failed to reconnect to new primary: %v", err)
			}
			defer conn.Close()

			client = pb.NewAuctionServiceClient(conn)
			stream, err = client.AuctionStream(context.Background())
			if err != nil {
				log.Fatalf("Failed to reconnect AuctionStream: %v", err)
			}
			log.Printf("Reconnected to new primary at: %s", newPrimary)

			// Resend initial message after reconnecting
			err = stream.Send(&pb.AuctionMessage{
				UserID:    clientInstance.ID,
				Timestamp: clientInstance.Clock.SendEvent(),
				Message:   "Reconnection message",
			})
			if err != nil {
				log.Printf("Error sending reconnection message: %v", err)
			}
		}

		// Process the incoming message
		if in != nil {
			clientInstance.Clock.ReceiveEvent(in.Timestamp)
			log.Printf("%v", in.Message)
		}
	}
}

func readInput() {
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
			getResult()
		case "help":
			log.Printf("result - To get the highest bid")
			log.Printf("bid <amount> - To bid on the auction")
		default:
			log.Printf("Unknow command: %v, use 'help' to get a list of the commands", command)
		}
	}
}

func getResult() {
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

func findPrimary(knownAddresses []string) string {
	retryInterval := 1 * time.Second
	maxRetries := 10

	for retries := 0; retries < maxRetries; retries++ {
		for _, address := range knownAddresses {
			conn, err := grpc.Dial(address, grpc.WithInsecure())
			if err != nil {
				log.Printf("Failed to connect to server at %s: %v", address, err)
				continue
			}

			client := pb.NewAuctionServiceClient(conn)
			resp, err := client.GetPrimary(context.Background(), &pb.PrimaryRequest{Timestamp: 1})
			conn.Close()

			if err == nil && resp != nil {
				log.Printf("Primary server found at %s (%s)", resp.Address, resp.StatusMessage)

				// Verify the primary is truly alive by testing a heartbeat
				if verifyPrimary(resp.Address) {
					return resp.Address
				}
			}

			log.Printf("Error calling GetPrimary on %s: %v", address, err)
		}

		log.Printf("Retrying to find primary in %v...", retryInterval)
		time.Sleep(retryInterval)
	}

	log.Fatalf("Failed to discover primary server after %d retries.", maxRetries)
	return ""
}

func verifyPrimary(address string) bool {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Printf("Failed to connect to verify primary at %s: %v", address, err)
		return false
	}
	defer conn.Close()

	client := pb.NewAuctionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // Timeout to avoid hanging
	defer cancel()

	_, err = client.SendHeartbeat(ctx, &pb.HeartbeatRequest{
		Timestamp: 2,
	})

	if err != nil {
		log.Printf("Failed to verify primary at %s: %v", address, err)
		return false
	}

	log.Printf("Primary at %s verified successfully.", address)
	return true
}
