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
	"time"
)

type Bidder struct {
	Stream pb.AuctionService_AuctionStreamServer
}

type Server struct {
	pb.UnimplementedAuctionServiceServer
	selfAddress          string
	Clock                *Clock.LClock
	ID                   int
	bidders              map[int32]*Bidder
	currentHighestBid    int32
	currentHighestBidder int32
	auctionEnded         bool
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
		Clock:   Clock.InitializeLClock(0),
		ID:      id,
		bidders: make(map[int32]*Bidder),
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

	if s.bidders[msg.UserID] == nil {
		log.Printf("User %d has joined the stream at lamport time: %d", msg.UserID, msg.Timestamp)
		s.bidders[msg.UserID] = &Bidder{Stream: stream}
	}

	for {
		msg, _ := stream.Recv()
		if err != nil {
			log.Printf(err.Error())
			return err
		}
		s.Clock.ReceiveEvent(msg.Timestamp)
		s.broadcastBid(&pb.BidRequest{
			Amount:   msg.Bid,
			BidderId: msg.UserID,
		})
	}
}

func (s *Server) broadcastBid(bidRequest *pb.BidRequest) {
	broadcastMessage := fmt.Sprintf("User %d has bid: %d at lamport time: %d", bidRequest.BidderId, bidRequest.Amount, bidRequest.Timestamp)
	bid := pb.AuctionMessage{
		Bid:       bidRequest.Amount,
		Timestamp: s.Clock.SendEvent(),
		UserID:    bidRequest.BidderId,
		Message:   broadcastMessage,
	}
	log.Printf(broadcastMessage)
	for _, bidder := range s.bidders {
		bidder.Stream.Send(&bid)
	}
}

func (s *Server) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	if req.Amount > s.currentHighestBid {
		s.currentHighestBid = req.Amount
		s.currentHighestBidder = req.BidderId
		s.broadcastBid(req)
		return &pb.BidResponse{
			Status:    "Success",
			Timestamp: s.Clock.SendEvent(),
		}, nil
	}
	return &pb.BidResponse{Status: "fail", Timestamp: s.Clock.SendEvent()}, nil
}

func formatBidMessage(message *pb.AuctionMessage) string {
	return fmt.Sprintf("Server has received request from user %d to bid %d at time %d", message.UserID, message.Bid, message.Timestamp)
}

func (s *Server) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	log.Printf("Server has received a result request at lamport time: %d", s.Clock.Time)

	return &pb.ResultResponse{
		AuctionEnded:  s.auctionEnded,
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func (s *Server) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	log.Printf("User: %d has left the auction at lamport time: %d", req.UserID, req.Timestamp)
	return &pb.LeaveResponse{Timestamp: s.Clock.SendEvent()}, nil
}

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	newUserID := int32(len(s.bidders) + 1) // Generate a new user ID
	s.bidders[newUserID] = &Bidder{}       // Add the new user to the bidders map

	log.Printf("User %d has joined the auction at lamport time: %d", newUserID, s.Clock.Time)

	return &pb.JoinResponse{
		UserID:    newUserID,
		Timestamp: s.Clock.SendEvent(),
	}, nil
}

func startAuctionTimer(s *Server, duration time.Duration) {
	time.Sleep(duration)
	s.auctionEnded = true
	log.Printf("Auction has ended at lamport time: %d", s.Clock.Time)
}
