package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
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
	auctionIsActive      bool
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

	if *isPrimary {
		log.Printf("Starting primary server at %s", selfAddress)
	} else {
		log.Printf("Starting backup server at %s (Primary is %s)", selfAddress, primaryAddress)
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
	}
	server := grpc.NewServer()
	pb.RegisterAuctionServiceServer(server, s)

	// Start gRPC server
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			log.Fatalf("Failed to listen on port %d: %v", *port, err)
		}
		log.Printf("Server is listening on port: %d", *port)
		if err := server.Serve(listener); err != nil {
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

	user, exists := s.bidders[msg.UserID]
	if !exists || user == nil {
		log.Printf("Registering new user %d for the stream at lamport time: %d", msg.UserID, s.Clock.Time)
		user = &Bidder{}
		s.bidders[msg.UserID] = user
	}
	s.Clock.Step()
	user.Stream = stream
	log.Printf("User %d is now connected to the stream at lamport time: %d", msg.UserID, s.Clock.Time)

	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("Error receiving message from user %d: %v", msg.UserID, err)
			return err
		}
		s.Clock.ReceiveEvent(msg.Timestamp)
		log.Printf("Received bid from user %d: %d at lamport time: %d", msg.UserID, msg.Bid, s.Clock.Time)

		// Broadcasting the bid to all users
		s.broadcastBid(&pb.BidRequest{
			Amount:   msg.Bid,
			BidderId: msg.UserID,
		})
	}
}

func (s *Server) broadcastBid(bidRequest *pb.BidRequest) {
	broadcastMessage := fmt.Sprintf("User %d has bid: %d at lamport time: %d", bidRequest.BidderId, bidRequest.Amount, s.Clock.Time)
	bid := pb.AuctionMessage{
		Bid:       bidRequest.Amount,
		Timestamp: s.Clock.SendEvent(),
		UserID:    bidRequest.BidderId,
		Message:   broadcastMessage,
	}
	log.Printf(broadcastMessage)
	for userID, bidder := range s.bidders {
		if bidder != nil && bidder.Stream != nil && userID != bidRequest.BidderId {
			log.Printf("Sending bid to user: %d", userID)
			err := bidder.Stream.Send(&bid)
			if err != nil {
				log.Printf("Error sending bid to user %d: %v", userID, err)
			}
		}
	}
}

func (s *Server) broadcastMessage(message pb.AuctionMessage) {
	log.Printf("Broadcasting a message to all users")
	for userID, bidder := range s.bidders {
		if bidder != nil && bidder.Stream != nil {
			err := bidder.Stream.Send(&message)
			if err != nil {
				log.Printf("Error sending bid to user %d: %v", userID, err)
			}
		}
	}
}

func (s *Server) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)

	if !s.auctionIsActive {
		go s.startAuctionTimer(30 * time.Second)
	}

	time.Sleep(1 * time.Second)

	if req.Amount > s.currentHighestBid && s.auctionIsActive {
		s.currentHighestBid = req.Amount
		s.currentHighestBidder = req.BidderId
		s.broadcastBid(req)
		return &pb.BidResponse{
			Success:   true,
			Timestamp: s.Clock.SendEvent(),
		}, nil
	}
	return &pb.BidResponse{Success: false, Timestamp: s.Clock.SendEvent()}, nil
}

func (s *Server) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	log.Printf("Server has received a result request at lamport time: %d", s.Clock.Time)

	return &pb.ResultResponse{
		AuctionEnded:  s.auctionIsActive,
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func (s *Server) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	log.Printf("User: %d has left the auction at lamport time: %d", req.UserID, s.Clock.Time)
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

	return &pb.JoinResponse{
		UserID:    newUserID,
		Timestamp: s.Clock.SendEvent(),
	}, nil
}

func (s *Server) startAuctionTimer(duration time.Duration) {
	s.auctionMutex.Lock()
	defer s.auctionMutex.Unlock()

	if s.auctionIsActive {
		log.Printf("Auction is already active")
		return
	}

	s.auctionIsActive = true
	s.Clock.Step()
	log.Printf("The auction was started at lamport: %d", s.Clock.Time)
	time.Sleep(duration)
	s.auctionIsActive = false
	s.Clock.Step()
	log.Printf("Auction has ended at lamport time: %d", s.Clock.Time)
	message := fmt.Sprintf("User: %d won the auction with bid: %d at lamport: %d", s.currentHighestBidder, s.currentHighestBid, s.Clock.Time)
	s.broadcastMessage(pb.AuctionMessage{
		Message:   message,
		UserID:    s.currentHighestBidder,
		Timestamp: s.Clock.SendEvent(),
		Bid:       s.currentHighestBid,
	})
	s.currentHighestBid = 0
	s.currentHighestBidder = 0
}

func (s *Server) GetPrimary(ctx context.Context, req *pb.Empty) (*pb.PrimaryResponse, error) {
	if s.isPrimary {
		log.Println("getPrimary called: This server is the primary.")
		return &pb.PrimaryResponse{
			Address:       s.selfAddress,
			StatusMessage: "This is the primary",
		}, nil
	}

	log.Printf("getPrimary called: Redirecting to known primary at %s", s.primaryAddress)
	return &pb.PrimaryResponse{
		Address:       s.primaryAddress,
		StatusMessage: "Redirect to primary",
	}, nil
}

func (s *Server) SendHeartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if !s.isPrimary {
		log.Println("Received heartbeat request, but this server is not the primary.")
		return &pb.HeartbeatResponse{IsAlive: false}, nil
	}

	log.Printf("Heartbeat received at timestamp: %d", req.Timestamp)
	return &pb.HeartbeatResponse{IsAlive: true}, nil
}

func (s *Server) monitorPrimary() {
	lastHeartbeat := time.Now()

	for {
		if s.isPrimary {
			return // Stop monitoring if this server becomes the primary
		}

		time.Sleep(1 * time.Second) // Check for heartbeats every second

		if time.Since(lastHeartbeat) > 5*time.Second { // Timeout of 5 seconds
			log.Println("Primary server is unresponsive. Promoting self to primary.")
			s.isPrimary = true
			s.primaryAddress = s.selfAddress
			return
		}

		conn, err := grpc.Dial(s.primaryAddress, grpc.WithInsecure())
		if err != nil {
			log.Printf("Failed to connect to primary for heartbeat check: %v", err)
			continue
		}

		client := pb.NewAuctionServiceClient(conn)
		resp, err := client.SendHeartbeat(context.Background(), &pb.HeartbeatRequest{
			Timestamp: int32(time.Now().Unix()),
		})
		conn.Close()

		if err != nil || !resp.IsAlive {
			log.Printf("Primary is unresponsive or failed heartbeat check: %v", err)
		} else {
			log.Println("Primary is alive")
			lastHeartbeat = time.Now() // Update the last heartbeat timestamp
		}
	}
}
