package server

import (
	"context"
	"flag"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"log"
)

type Server struct {
	pb.UnimplementedAuctionServiceServer
	selfAddress          string
	Clock                *Clock.LClock
	ID                   int
	bidders              map[int32](pb.AuctionService_AuctionStreamServer)
	currentHighestBid    int32
	currentHighestBidder int32
}

/*
Sets up a server node. You must specify via flag whether it's the primary or not, by calling it with flag -isPrimary true/false
Server nodes reserve ports 5000 and 5001.
*/
func main() {
	isPrimary := *flag.Bool("isPrimary", false, "Is the node the primary server?")
	flag.Parse()
	var selfAddress string
	var id int
	if isPrimary {
		selfAddress = fmt.Sprintf("127.0.0.1:5000")
		id = 0
	} else {
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
	if bidRequest.Amount > s.currentHighestBid {
		s.currentHighestBid = bidRequest.Amount
		s.currentHighestBidder = bidRequest.BidderId
	}

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

func formatBidMessage(message *pb.AuctionMessage) string {
	return fmt.Sprintf("Server has received request from user %d to bid %d at time %d", message.UserID, message.Bid, message.Timestamp)
}

func (s *Server) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)

	return &pb.ResultResponse{
		AuctionEnded:  false, // WE NEED TO SET THIS WHEN WE GET OUR AUCTION LOGIC IN PLACE
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}
