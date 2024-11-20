package server

import (
	"flag"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
	"google.golang.org/grpc"
	"log"
)

type Server struct {
	pb.UnimplementedAuctionServiceServer
	selfAddress string
	Clock       *Clock.LClock
	ID          int
	bidders     map[int32](pb.AuctionService_AuctionStreamServer)
}

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
		Clock: Clock.InitializeLClock(0),
		ID: id,
		bidders: make(map[int32](pb.AuctionService_AuctionStreamServer)),
	})

}

func (s *Server) AuctionStream(stream pb.AuctionService_AuctionStreamServer) error {
	msg, err := stream.Recv()
	if err != nil {
		log.Println(err.Error())
	}
	if s.bidders[msg.UserID] == nil {
		s.bidders[msg.UserID] = stream
	}

	for{

	}

}

func (s *Server) broadcastBid(bidRequest pb.BidRequest){

	bid := pb.AuctionStream{
		HighestBid: 0,
		AuctionEnded: false,
		HighestBidder: "",
		Message: bidRequest.
	}

	for _, stream  := range s.bidders {
		stream.Send()
	}

}
