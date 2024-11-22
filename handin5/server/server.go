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
	Clock                *Clock.LClock
	ID                   int
	bidders              map[int32]*Bidder
	currentHighestBid    int32
	currentHighestBidder int32
	auctionIsActive      bool
}

var port *int = flag.Int("Port", 1337, "Server Port")

/*
Sets up a server node. You must specify via flag whether it's the primary or not, by calling it with flag -isPrimary true/false
*/
func main() {
	isPrimary := *flag.Bool("isPrimary", false, "Is the node the primary server?")
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	id := 0
	if !isPrimary {
		id = 1
	}

	server := grpc.NewServer()

	pb.RegisterAuctionServiceServer(server, &Server{
		Clock:           Clock.InitializeLClock(0),
		ID:              id,
		bidders:         make(map[int32]*Bidder),
		auctionIsActive: false,
	})
	log.Printf("Server is listening on port: %d", *port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

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
	// TO-DO we need to broadcast to all users.
}
