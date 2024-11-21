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

type Server struct {
	pb.UnimplementedAuctionServiceServer
	selfAddress          string
	Clock                *Clock.LClock
	ID                   int
	bidders              map[int32](pb.AuctionService_AuctionStreamServer)
	currentHighestBid    int32
	currentHighestBidder int32
	auctionEnded         bool
}

var port *int = flag.Int("Port", 1337, "Server Port")

/*
Sets up a server node. You must specify via flag whether it's the primary or not, by calling it with flag -isPrimary true/false
Server nodes reserve ports 5000 and 5001.
*/
func main() {
	isPrimary := *flag.Bool("isPrimary", false, "Is the node the primary server?")
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	selfAddress := fmt.Sprintf("127.0.0.1:5000")
	id := 0
	if !isPrimary {
		selfAddress = fmt.Sprintf("127.0.0.1:5001")
		id = 1
	}

	server := grpc.NewServer()

	pb.RegisterAuctionServiceServer(server, &Server{
		selfAddress: selfAddress,
		Clock:       Clock.InitializeLClock(0),
		ID:          id,
		bidders:     make(map[int32]pb.AuctionService_AuctionStreamServer),
	})
	log.Printf("Server is listening on port: %d", *port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

}

/*
 */
func (s *Server) AuctionStream(stream pb.AuctionService_AuctionStreamServer) error {
	msg, err := stream.Recv()
	s.Clock.ReceiveEvent(msg.Timestamp)

	if err != nil {
		log.Println(err.Error())
	}
	if s.bidders[msg.UserID] == nil {
		log.Printf("User %d has joined the stream at time with bid %d")
		s.bidders[msg.UserID] = stream
	}

	for {
		msg, _ := stream.Recv()
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
	for _, stream := range s.bidders {
		stream.Send(&bid)
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
		AuctionEnded:  false, // WE NEED TO SET THIS WHEN WE GET OUR AUCTION LOGIC IN PLACE
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func startAuctionTimer(s *Server, duration time.Duration) {
	time.Sleep(duration)
	s.auctionEnded = true
	log.Printf("Auction has ended at lamport time: %d", s.Clock.Time)
}
