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
	Clock   *Clock.LClock
	ID      int32
	logFile *log.Logger
}

var clientInstance Client
var client pb.AuctionServiceClient

func main() {
	logFile := logs.InitLogger("Client" + strconv.Itoa(os.Getpid()))
	knownAddresses := []string{"localhost:1337", "localhost:1338"} // List of known servers
	primaryAddress, time := findPrimary(knownAddresses, logFile)   // Find the current primary server

	conn, err := grpc.DialContext(context.Background(), primaryAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User failed connecting to auction: %v", err)
	}
	defer conn.Close()
	clientClock := Clock.InitializeLClock(time)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Trying to connect to the auction at lamport: %v", clientClock.Time)
	logs.WriteToLog(logFile, "Trying to connect to the auction", clientClock.Time, -1) //No user ID yet.
	client = pb.NewAuctionServiceClient(conn)
	response, err := client.Join(context.Background(), &pb.JoinRequest{Timestamp: clientClock.Time})
	if err != nil {
		log.Fatalf("User failed to join the AuctionStream: %v", err)
	}
	clientClock.ReceiveEvent(response.Timestamp)
	clientInstance = Client{ID: response.UserID, Clock: clientClock, logFile: logFile}

	logs.WriteToLog(clientInstance.logFile, "Received join confirmation from server", clientInstance.Clock.Time, clientInstance.ID)

	clientInstance.Clock.Step()
	logs.WriteToLog(clientInstance.logFile, "Attempting to open stream", clientInstance.Clock.Time, clientInstance.ID)

	stream, err := client.AuctionStream(context.Background())
	if err != nil {
		clientInstance.Clock.Step()
		logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Failed to open AuctionStream: %v", err), clientInstance.Clock.Time, clientInstance.ID)
		log.Fatalf("Failed to open AuctionStream: %v", err)
	}

	clientInstance.Clock.Step()
	logs.WriteToLog(clientInstance.logFile, "Attempting to send initial connection message", clientInstance.Clock.Time, clientInstance.ID)

	err = stream.Send(&pb.AuctionMessage{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.Time,
		Message:   "Initial connection message",
	})
	if err != nil {
		clientInstance.Clock.Step()
		log.Printf("Error sending initial message: %v", err)
		logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Error sending initial message: %v", err), clientInstance.Clock.Time, clientInstance.ID)

	}

	go listenToStream(stream, knownAddresses)
	go readInput()

	<-sigs

	clientInstance.Clock.Step()
	logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Sending leave message"), clientInstance.Clock.Time, clientInstance.ID)

	LeaveResponse, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: clientInstance.ID, Timestamp: clientInstance.Clock.Time})
	if err != nil {
		clientInstance.Clock.Step()
		log.Printf("User: %d failed to leave the AuctionStream", clientInstance.ID)
		logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("User failed to leave the AuctionStream", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)

	}
	clientInstance.Clock.ReceiveEvent(LeaveResponse.Timestamp)
	stream.CloseSend()
	logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("User successfully left the auction!", clientInstance.ID), clientInstance.Clock.Time, clientInstance.ID)
	log.Printf("User: %d successfully left the auction!", clientInstance.ID)
}

func listenToStream(stream pb.AuctionService_AuctionStreamClient, knownAddresses []string) {
	for {
		in, err := stream.Recv()
		if err != nil {
			clientInstance.Clock.Step()
			if err == io.EOF {
				log.Printf("Server closed the stream.")
				logs.WriteToLog(clientInstance.logFile, "Server closed the stream.", clientInstance.Clock.Time, clientInstance.ID)
				return
			}
			log.Printf("Error while receiving message: %v", err)
			logs.WriteToLog(clientInstance.logFile, "Attempting to reconnect to the auction...", clientInstance.Clock.Time, clientInstance.ID)
			// Handle reconnection logic
			log.Printf("Attempting to reconnect to the auction...")
			time.Sleep(6 * time.Second)                                          // Temp solution, as the backup server needs to assign itself as primary before we can make the call
			newPrimary, _ := findPrimary(knownAddresses, clientInstance.logFile) //We know client has already been initialized
			conn, err := grpc.DialContext(context.Background(), newPrimary, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				log.Fatalf("Failed to reconnect to new primary: %v", err)
			}
			defer conn.Close()

			client = pb.NewAuctionServiceClient(conn)
			stream, err = client.AuctionStream(context.Background())
			clientInstance.Clock.Step()
			if err != nil {
				log.Fatalf("Failed to reconnect AuctionStream: %v", err)
				logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Failed to reconnect AuctionStream: %v", err), clientInstance.Clock.Time, clientInstance.ID)

			}
			log.Printf("Reconnected to new primary at: %s", newPrimary)
			logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Reconnected to new primary at: %s", newPrimary), clientInstance.Clock.Time, clientInstance.ID)

			// Resend initial message after reconnecting
			clientInstance.Clock.Step()
			logs.WriteToLog(clientInstance.logFile, "Resending initial message", clientInstance.Clock.Time+1, clientInstance.ID)

			err = stream.Send(&pb.AuctionMessage{
				UserID:    clientInstance.ID,
				Timestamp: clientInstance.Clock.Time,
				Message:   "Reconnection message",
			})
			if err != nil {
				log.Printf("Error sending reconnection message: %v", err)
			}
		}

		// Process the incoming message
		if in != nil {
			clientInstance.Clock.ReceiveEvent(in.Timestamp)
			logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Received message %s", in.Message), clientInstance.Clock.Time, clientInstance.ID)
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
			clientInstance.Clock.Step()
			logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Sending a bid to the server of amount %d", bid), clientInstance.Clock.Time, clientInstance.ID)
			log.Printf("Sending a bid to the server at lamport time: %d", clientInstance.Clock.Time)

			response, err := client.Bid(context.Background(), &pb.BidRequest{Amount: bid, BidderId: clientInstance.ID, Timestamp: clientInstance.Clock.Time})

			if err != nil {
				log.Printf("Error sending bid: %v", err)
				continue
			}
			clientInstance.Clock.ReceiveEvent(response.Timestamp)
			logs.WriteToLog(clientInstance.logFile, "Received answer for bid request from server.", clientInstance.Clock.Time, clientInstance.ID)

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
			log.Printf("Unknown command: %v, use 'help' to get a list of the commands", command)
		}
	}
}

func getResult() {
	clientInstance.Clock.Step()
	logs.WriteToLog(clientInstance.logFile, "Requesting current result from server", clientInstance.Clock.Time+1, clientInstance.ID)
	response, err := client.Result(context.Background(), &pb.ResultRequest{
		UserID:    clientInstance.ID,
		Timestamp: clientInstance.Clock.Time,
	})
	if err != nil {
		log.Printf("Getting the result caused an error: %v", err)
		return
	}
	clientInstance.Clock.ReceiveEvent(response.Timestamp)
	logs.WriteToLog(clientInstance.logFile, "Received current result from server", clientInstance.Clock.Time, clientInstance.ID)

	if response.AuctionEnded {
		log.Printf("Highest bid was: %d by user: %d", response.HighestBid, response.HighestBidder)
	} else {
		log.Printf("Current highest bid is: %d by user: %d", response.HighestBid, response.HighestBidder)
	}
}

func findPrimary(knownAddresses []string, logFile *log.Logger) (string, int32) {
	retryInterval := 1 * time.Second
	maxRetries := 10

	for retries := 0; retries < maxRetries; retries++ {
		for _, address := range knownAddresses {
			conn, err := grpc.Dial(address, grpc.WithInsecure())
			if err != nil {
				log.Printf("Failed to connect to server at %s: %v", address, err)
				continue
			}
			var resp *pb.PrimaryResponse
			client := pb.NewAuctionServiceClient(conn)
			if CheckZeroInitialized(&clientInstance) {
				logs.WriteToLog(logFile, "Attempting to get primary", 0, -1)
				resp, err = client.GetPrimary(context.Background(), &pb.PrimaryRequest{Timestamp: 0})
			} else {
				clientInstance.Clock.Step()
				logs.WriteToLog(logFile, "Attempting to get primary", clientInstance.Clock.Time, clientInstance.ID)
				resp, err = client.GetPrimary(context.Background(), &pb.PrimaryRequest{Timestamp: clientInstance.Clock.Time})
			}
			conn.Close()

			if err == nil && resp != nil {
				log.Printf("Primary server found at %s (%s)", resp.Address, resp.StatusMessage)
				if CheckZeroInitialized(&clientInstance) {
					logs.WriteToLog(logFile, fmt.Sprintf("Primary server found at %s (%s)", resp.Address, resp.StatusMessage), resp.Timestamp, -1)
				} else {
					clientInstance.Clock.ReceiveEvent(resp.Timestamp)
					logs.WriteToLog(logFile, fmt.Sprintf("Primary server found at %s (%s)", resp.Address, resp.StatusMessage), resp.Timestamp, -1)
				}
				// Verify the primary is truly alive by testing a heartbeat
				if verifyPrimary(resp.Address, logFile) {
					if CheckZeroInitialized(&clientInstance) {
						return resp.Address, resp.Timestamp + 1

					}
					return resp.Address, clientInstance.Clock.Time
				}
			}

			log.Printf("Error calling GetPrimary on %s: %v", address, err)
		}

		log.Printf("Retrying to find primary in %v...", retryInterval)
		time.Sleep(retryInterval)
	}

	log.Fatalf("Failed to discover primary server after %d retries.", maxRetries)
	return "", 0
}

func verifyPrimary(address string, logFile *log.Logger) bool {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Printf("Failed to connect to verify primary at %s: %v", address, err)
		return false
	}
	defer conn.Close()

	client := pb.NewAuctionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // Timeout to avoid hanging
	defer cancel()

	//Should we increment timestamps here?
	/*
		if CheckZeroInitialized(&clientInstance) {
			logs.WriteToLog(logFile, "Verifying primary", 2, -1) //Ikke helt sikker på hvorfor den her skal være 2.
		} else {
			logs.WriteToLog(logFile, "Verifying primary", clientInstance.Clock.Time, clientInstance.ID)
		}
	*/

	_, err = client.SendHeartbeat(ctx, &pb.HeartbeatRequest{
		Timestamp: 2,
	})

	if err != nil {
		log.Printf("Failed to verify primary at %s: %v", address, err)
		return false
	}

	/*
		If it is zero initialized, we want to call receive event on whatever we get back from the server
	*/
	/*
		tempClock := Clock.InitializeLClock(2)
		tempClock.ReceiveEvent(response.LamportClock)

		if CheckZeroInitialized(&clientInstance) {
			logs.WriteToLog(logFile, fmt.Sprintf("Primary at %s verified successfully.", address), tempClock.Time, clientInstance.ID) //Ikke helt sikker på hvorfor den her skal være 2.
		} else {
			logs.WriteToLog(logFile, fmt.Sprintf("Primary at %s verified successfully.", address), clientInstance.Clock.Time, clientInstance.ID)
		}
	*/

	log.Printf("Primary at %s verified successfully.", address)
	//logs.WriteToLog(clientInstance.logFile, fmt.Sprintf("Primary at %s verified successfully.", address), clientInstance.Clock.Time, clientInstance.ID)

	return true
}
func CheckZeroInitialized(c *Client) bool {
	if c == nil {
		return true
	}
	return c.logFile == nil || c.Clock == nil
}
