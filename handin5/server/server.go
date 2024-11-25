package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5/logs"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Bidder struct {
	Stream pb.AuctionService_AuctionStreamServer
}

type Server struct {
	pb.UnimplementedAuctionServiceServer
	auctionMutex         sync.Mutex
	selfAddress          string
	primaryAddress       string
	isPrimary            bool
	Clock                *Clock.LClock
	ID                   int
	bidders              map[int32]*Bidder
	currentHighestBid    int32
	currentHighestBidder int32
	remainingTime        int32
	auctionIsActive      bool
	logFile              *log.Logger
}

/*
Sets up a server node. You must specify via flag whether it's the primary or not, by calling it with flag -isPrimary true/false
*/
func main() {
	isPrimary := flag.Bool("isPrimary", false, "Is the node the primary server?")
	port := flag.Int("Port", 1337, "Server Port")
	flag.Parse()

	selfAddress := fmt.Sprintf("localhost:%d", *port)
	primaryAddress := "localhost:1337"

	//Create log file.
	logFile := logs.InitServerLogger(*isPrimary)

	if *isPrimary {
		log.Printf("Starting primary server at %s", selfAddress)
		logs.WriteToServerLog(logFile, fmt.Sprintf("Starting primary server at %s", selfAddress), 0)
	} else {
		log.Printf("Starting backup server at %s (Primary is %s)", selfAddress, primaryAddress)
		logs.WriteToServerLog(logFile, fmt.Sprintf("Starting backup server at %s", selfAddress), 0)

	}

	id := 0
	if !*isPrimary {
		id = 1
	}

	s := &Server{
		Clock:           Clock.InitializeLClock(0),
		ID:              id,
		bidders:         make(map[int32]*Bidder),
		auctionIsActive: false,
		selfAddress:     selfAddress,
		primaryAddress:  primaryAddress,
		isPrimary:       *isPrimary,
		logFile:         logFile,
	}
	server := grpc.NewServer()
	pb.RegisterAuctionServiceServer(server, s)

	s.Clock.Step()
	// Start gRPC server
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			log.Fatalf("Failed to listen on port %d: %v", *port, err)
		}
		log.Printf("Server is listening on port: %d", *port)
		logs.WriteToServerLog(logFile, fmt.Sprintf("Server is listening on port: %d", *port), s.Clock.Time)

		if err := server.Serve(listener); err != nil {
			logs.WriteToServerLog(logFile, fmt.Sprintf("Failed to serve: %v", err), s.Clock.Time)
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	go s.monitorPrimary()

	select {}
}

func (s *Server) AuctionStream(stream pb.AuctionService_AuctionStreamServer) error {
	msg, err := stream.Recv()
	if err != nil {
		log.Println(err.Error())
		return err
	}

	s.Clock.ReceiveEvent(msg.Timestamp)
	log.Printf("First messages was received from the stream to establish a connection at lamport: %d", s.Clock.Time)
	logs.WriteToServerLog(s.logFile, "First messages was received from the stream to establish a connection", s.Clock.Time)

	s.auctionMutex.Lock()
	user, exists := s.bidders[msg.UserID]
	if !exists || user == nil {
		s.Clock.Step()
		log.Printf("Registering new user %d for the stream at lamport time: %d", msg.UserID, s.Clock.Time)
		logs.WriteToServerLog(s.logFile, fmt.Sprintf("Registering new user %d", msg.UserID), s.Clock.Time)
		user = &Bidder{}
		s.bidders[msg.UserID] = user
	}
	s.Clock.Step()
	user.Stream = stream
	s.auctionMutex.Unlock()

	log.Printf("User %d is now connected to the stream at lamport time: %d", msg.UserID, s.Clock.Time)
	logs.WriteToServerLog(s.logFile, fmt.Sprintf("User %d is now connected to the stream.", msg.UserID), s.Clock.Time)

	for {
		msg, err := stream.Recv()
		if err != nil {
			s.Clock.Step()
			if err == io.EOF {
				log.Printf("Stream closed for user with error: %v", err)
				logs.WriteToServerLog(s.logFile, fmt.Sprintf("Stream closed for user with error: %v", err), s.Clock.Time)

				break
			}
			log.Printf("Error receiving message from user %v", err)
			logs.WriteToServerLog(s.logFile, fmt.Sprintf("Error receiving message from user %v", err), s.Clock.Time)

			break
		}
		s.Clock.ReceiveEvent(msg.Timestamp)
		log.Printf("Received bid from user %d: %d at lamport time: %d", msg.UserID, msg.Bid, s.Clock.Time)
		logs.WriteToServerLog(s.logFile, fmt.Sprintf("Received bid from user %d: %d", msg.UserID, msg.Bid), s.Clock.Time)

		// Broadcasting the bid to all users
		s.broadcastBid(&pb.BidRequest{
			Amount:   msg.Bid,
			BidderId: msg.UserID,
		})

		// Cleaning up when the user leaves the stream
		s.auctionMutex.Lock()
		if _, exists := s.bidders[msg.UserID]; exists {
			delete(s.bidders, msg.UserID)
			s.Clock.Step()
			log.Printf("User %d has been removed from the auction at lamport time: %d", msg.UserID, s.Clock.Time)
			logs.WriteToServerLog(s.logFile, fmt.Sprintf("User %d has been removed from the auction", msg.UserID), s.Clock.Time)

		}
		s.auctionMutex.Unlock()
	}
	return nil
}

func (s *Server) broadcastBid(bidRequest *pb.BidRequest) {
	s.Clock.Step()
	broadcastMessage := fmt.Sprintf("User %d has bid: %d at lamport time: %d", bidRequest.BidderId, bidRequest.Amount, s.Clock.Time)

	bid := pb.AuctionMessage{
		Bid:       bidRequest.Amount,
		Timestamp: s.Clock.Time,
		UserID:    bidRequest.BidderId,
		Message:   broadcastMessage,
	}
	log.Printf(broadcastMessage)
	for userID, bidder := range s.bidders {
		if bidder != nil && bidder.Stream != nil && userID != bidRequest.BidderId {
			log.Printf("Sending bid to user: %d", userID)
			logs.WriteToServerLog(s.logFile, fmt.Sprintf("Sending bid to user: %d", userID), s.Clock.Time)

			err := bidder.Stream.Send(&bid)
			if err != nil {
				log.Printf("Error sending bid to user %d: %v", userID, err)
				logs.WriteToServerLog(s.logFile, fmt.Sprintf("Error sending bid to user %d: %v", userID, err), s.Clock.Time)

			}
		}
	}
}

func (s *Server) broadcastMessage(message pb.AuctionMessage) {
	s.Clock.Step()
	log.Printf("Broadcasting a message to all users")
	logs.WriteToServerLog(s.logFile, "Broadcasting a message to all users", s.Clock.Time)

	for userID, bidder := range s.bidders {
		if bidder != nil && bidder.Stream != nil {
			err := bidder.Stream.Send(&message)
			if err != nil {
				log.Printf("Error sending bid to user %d: %v", userID, err)
				logs.WriteToServerLog(s.logFile, fmt.Sprintf("Error sending bid to user %d: %v", userID, err), s.Clock.Time)

			}
		}
	}
}

func (s *Server) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	logs.WriteToServerLog(s.logFile, fmt.Sprintf("Received bid request from user %d: %d", req.BidderId, req.Amount), s.Clock.Time)

	if !s.auctionIsActive {
		go s.setUpAuction()
	}

	time.Sleep(1 * time.Second)

	if req.Amount > s.currentHighestBid && s.auctionIsActive {
		s.currentHighestBid = req.Amount
		s.currentHighestBidder = req.BidderId
		s.Clock.Step()
		logs.WriteToServerLog(s.logFile, "Broadcasting bid", s.Clock.Time)
		s.broadcastBid(req)
		logs.WriteToServerLog(s.logFile, "Returning response to bidder", s.Clock.Time)

		return &pb.BidResponse{
			Success:   true,
			Timestamp: s.Clock.Time,
		}, nil
	}
	return &pb.BidResponse{Success: false, Timestamp: s.Clock.Time}, nil
}

func (s *Server) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	log.Printf("Server has received a result request at lamport time: %d", s.Clock.Time)
	logs.WriteToServerLog(s.logFile, "Server has received a result request.", s.Clock.Time)

	return &pb.ResultResponse{
		AuctionEnded:  s.auctionIsActive,
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func (s *Server) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	logs.WriteToServerLog(s.logFile, fmt.Sprintf("Received leave request from user %d", req.UserID), s.Clock.Time)

	s.auctionMutex.Lock()
	defer s.auctionMutex.Unlock()

	if _, exists := s.bidders[req.UserID]; exists {
		delete(s.bidders, req.UserID) // Remove the user from the map
		log.Printf("User: %d has left from the auction at lamport time: %d", req.UserID, s.Clock.Time)
	} else {
		log.Printf("Leave called for non-existent user: %d", req.UserID)
	}

	return &pb.LeaveResponse{Timestamp: s.Clock.SendEvent()}, nil
}

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)

	newUserID := int32(len(s.bidders) + 1)
	if _, exists := s.bidders[newUserID]; !exists {
		log.Printf("Registering new user %d in the bidders map.", newUserID)
		s.bidders[newUserID] = &Bidder{}
	} else {
		log.Printf("User %d already exists in the bidders map.", newUserID)
	}

	log.Printf("User %d has joined the auction at lamport time: %d", newUserID, s.Clock.Time)
	logs.WriteToServerLog(s.logFile, fmt.Sprintf("User %d has joined the auction", newUserID), s.Clock.Time)
	s.Clock.Step()
	logs.WriteToServerLog(s.logFile, fmt.Sprintf("Sending unique ID to user", newUserID), s.Clock.Time)

	return &pb.JoinResponse{
		UserID:    newUserID,
		Timestamp: s.Clock.Time,
	}, nil
}

func (s *Server) setUpAuction() {
	s.Clock.Step()

	if s.auctionIsActive {
		log.Printf("Auction is already active!")
		s.startAuctionTimer()
		return
	}

	s.auctionIsActive = true
	var timer int32 = 30
	s.remainingTime = timer
	log.Printf("The auction was started at lamport: %d", s.Clock.Time)
	logs.WriteToServerLog(s.logFile, "The auction was started", s.Clock.Time)

	s.startAuctionTimer()
}

func (s *Server) startAuctionTimer() {
	for s.remainingTime != 0 {
		time.Sleep(time.Second)
		s.remainingTime--
		s.Clock.Step()
		logs.WriteToServerLog(s.logFile, fmt.Sprintf("Auction has %d seconds remaining", s.remainingTime), s.Clock.Time)

	}
	s.Clock.Step()
	log.Printf("Auction has ended at lamport time: %d", s.Clock.Time)
	logs.WriteToServerLog(s.logFile, "Auction has ended", s.Clock.Time)

	message := fmt.Sprintf("User: %d won the auction with bid: %d", s.currentHighestBidder, s.currentHighestBid)
	s.broadcastMessage(pb.AuctionMessage{
		Message:   message,
		UserID:    s.currentHighestBidder,
		Timestamp: s.Clock.SendEvent(),
		Bid:       s.currentHighestBid,
	})
	s.currentHighestBid = 0
	s.currentHighestBidder = 0
	s.auctionIsActive = false
}

func (s *Server) GetPrimary(ctx context.Context, req *pb.PrimaryRequest) (*pb.PrimaryResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	logs.WriteToServerLog(s.logFile, "getPrimary called", s.Clock.Time)

	if s.isPrimary {
		log.Println("getPrimary called: This server is the primary.")
		//logs.WriteToServerLog(s.logFile, fmt.Sprintf("Returning that node is primary", s.primaryAddress), s.Clock.Time+1)

		return &pb.PrimaryResponse{
			Address:       s.selfAddress,
			StatusMessage: "This is the primary",
			Timestamp:     s.Clock.SendEvent(),
		}, nil
	}

	log.Printf("getPrimary called: Redirecting to known primary at %s", s.primaryAddress)
	//logs.WriteToServerLog(s.logFile, fmt.Sprintf("getPrimary called: Redirecting to known primary at %s", s.primaryAddress), s.Clock.Time+1)

	return &pb.PrimaryResponse{
		Address:       s.primaryAddress,
		StatusMessage: "Redirect to primary",
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func (s *Server) SendHeartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if !s.isPrimary {
		log.Println("Received heartbeat request, but this server is not the primary.")
		//logs.WriteToServerLog(s.logFile, "Received heartbeat request, but this server is not the primary.", s.Clock.Time)

		return &pb.HeartbeatResponse{IsAlive: false}, nil
	}

	//logs.WriteToServerLog(s.logFile, "Received heartbeat request", s.Clock.Time)

	return &pb.HeartbeatResponse{
		IsAlive:              true,
		CurrentHighestBid:    s.currentHighestBid,
		CurrentHighestBidder: s.currentHighestBidder,
		RemainingTime:        s.remainingTime,
		AuctionIsActive:      s.auctionIsActive,
		LamportClock:         s.Clock.Time,
	}, nil
}

func (s *Server) monitorPrimary() {
	lastHeartbeat := time.Now()

	for {
		if s.isPrimary {
			return // Stop monitoring if this server becomes the primary
		}

		time.Sleep(1 * time.Second) // Check for heartbeats every second

		if time.Since(lastHeartbeat) > 5*time.Second { // Timeout of 5 seconds
			s.Clock.Step()
			log.Println("Primary server is unresponsive. Promoting self to primary.")
			logs.WriteToServerLog(s.logFile, "Primary server is unresponsive. Promoting self to primary.", s.Clock.Time)

			s.isPrimary = true
			s.primaryAddress = s.selfAddress

			// Initialize auction if it's active on the failed primary
			if s.remainingTime > 0 {
				log.Println("Continuing auction from backup server.")
				//logs.WriteToServerLog(s.logFile, "Continuing auction from backup server.", s.Clock.Time)

				go s.setUpAuction() // Resume auction with remaining time
			} else {
				log.Println("No active auction to continue.")
				//logs.WriteToServerLog(s.logFile, "No active auction to continue.", s.Clock.Time)

			}
			return
		}

		conn, err := grpc.Dial(s.primaryAddress, grpc.WithInsecure())
		if err != nil {
			log.Printf("Failed to connect to primary for heartbeat check: %v", err)
			//logs.WriteToServerLog(s.logFile, fmt.Sprintf("Failed to connect to primary for heartbeat check: %v", err), s.Clock.Time)

			continue
		}

		client := pb.NewAuctionServiceClient(conn)
		resp, err := client.SendHeartbeat(context.Background(), &pb.HeartbeatRequest{
			Timestamp: int32(time.Now().Unix()),
		})
		conn.Close()

		if err != nil || !resp.IsAlive {
			log.Printf("Primary is unresponsive or failed heartbeat check: %v", err)
			//logs.WriteToServerLog(s.logFile, fmt.Sprintf("Primary is unresponsive or failed heartbeat check: %v", err), s.Clock.Time)

		} else {
			lastHeartbeat = time.Now() // Update the last heartbeat timestamp
			if resp.AuctionIsActive {
				s.Clock.ReceiveEvent(resp.LamportClock)
				s.currentHighestBidder = resp.CurrentHighestBidder
				s.currentHighestBid = resp.CurrentHighestBid
				s.remainingTime = resp.RemainingTime
				s.auctionIsActive = resp.AuctionIsActive
			}
		}
	}
}
