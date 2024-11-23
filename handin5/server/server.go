package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5/logs"
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
	log                  *log.Logger
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
		log:             logs.InitLogger("Server"),
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
	logs.WriteToLog(s.log, "First messages was received from the stream to establish a connection", s.Clock.Time, int32(s.ID))

	user, exists := s.bidders[msg.UserID]
	if !exists || user == nil {
		logs.WriteToLog(s.log, fmt.Sprintf("Registering new user %d for the stream at lamport time:", msg.UserID), s.Clock.Time, int32(s.ID))
		user = &Bidder{}
		s.bidders[msg.UserID] = user
	}
	s.Clock.Step()
	user.Stream = stream
	logs.WriteToLog(s.log, fmt.Sprintf("User %d is now connected to the stream", msg.UserID), s.Clock.Time, int32(s.ID))

	for {
		msg, err := stream.Recv()
		if err != nil {
			logs.WriteToLog(s.log, fmt.Sprintf("Error receiving message from user %d: %v", msg.UserID, err), s.Clock.Time, int32(s.ID))
			return err
		}
		s.Clock.ReceiveEvent(msg.Timestamp)
		log.Printf("Received bid from user %d: %d at lamport time: %d", msg.UserID, msg.Bid, s.Clock.Time)
		logs.WriteToLog(s.log, fmt.Sprintf("Received bid grom user %d", msg.UserID), s.Clock.Time, int32(s.ID))
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
			logs.WriteToLog(s.log, fmt.Sprintf("Sending bid to user: %d", bidder), s.Clock.Time, int32(s.ID))
			err := bidder.Stream.Send(&bid)
			if err != nil {
				logs.WriteToLog(s.log, fmt.Sprintf("Error sending bid to user %d: %v", userID, err), s.Clock.Time, int32(s.ID))

			}
		}
	}
}

func (s *Server) broadcastMessage(message pb.AuctionMessage) {
	log.Printf("Broadcasting a message to all users")
	logs.WriteToLog(s.log, "Broadcasting a message to all users", s.Clock.Time, int32(s.ID))
	for userID, bidder := range s.bidders {
		if bidder != nil && bidder.Stream != nil {
			err := bidder.Stream.Send(&message)
			if err != nil {
				logs.WriteToLog(s.log, fmt.Sprintf("Error sending bid to user %d", userID), s.Clock.Time, int32(s.ID))
			}
		}
	}
}

func (s *Server) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	logs.WriteToLog(s.log, fmt.Sprintf("Received bid from user %d", req.BidderId), s.Clock.Time, int32(s.ID))

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
	logs.WriteToLog(s.log, fmt.Sprintf("Received result from user %d", s.ID), s.Clock.Time, int32(s.ID))
	return &pb.ResultResponse{
		AuctionEnded:  s.auctionIsActive,
		HighestBidder: s.currentHighestBidder,
		HighestBid:    s.currentHighestBid,
		Timestamp:     s.Clock.SendEvent(),
	}, nil
}

func (s *Server) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)
	logs.WriteToLog(s.log, fmt.Sprintf("Received leave from user %d", s.ID), s.Clock.Time, int32(s.ID))
	return &pb.LeaveResponse{Timestamp: s.Clock.SendEvent()}, nil
}

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.Clock.ReceiveEvent(req.Timestamp)

	newUserID := int32(len(s.bidders) + 1)
	if _, exists := s.bidders[newUserID]; !exists {
		logs.WriteToLog(s.log, fmt.Sprintf("Registering new user %d", newUserID), s.Clock.Time, int32(s.ID))
		s.bidders[newUserID] = &Bidder{}
	} else {
		logs.WriteToLog(s.log, fmt.Sprintf("User %d already exists in the bidders map.", newUserID), s.Clock.Time, int32(s.ID))
	}

	logs.WriteToLog(s.log, fmt.Sprintf("User %d has joined the auction at lamport time: %d", newUserID), s.Clock.Time, int32(s.ID))
	return &pb.JoinResponse{
		UserID:    newUserID,
		Timestamp: s.Clock.SendEvent(),
	}, nil
}

func (s *Server) startAuctionTimer(duration time.Duration) {
	s.auctionMutex.Lock()
	defer s.auctionMutex.Unlock()

	if s.auctionIsActive {
		logs.WriteToLog(s.log, "Auction is already active", s.Clock.Time, int32(s.ID))
		return
	}

	s.auctionIsActive = true
	s.Clock.Step()
	logs.WriteToLog(s.log, "Auction was started", s.Clock.Time, int32(s.ID))
	time.Sleep(duration)
	s.auctionIsActive = false
	s.Clock.Step()
	logs.WriteToLog(s.log, "Auction has ended", s.Clock.Time, int32(s.ID))

	message := fmt.Sprintf("User: %d won the auction with bid: %d at lamport: %d", s.currentHighestBidder, s.currentHighestBid, s.Clock.Time)
	logs.WriteToLog(s.log, message, s.Clock.Time, int32(s.ID))
	s.broadcastMessage(pb.AuctionMessage{
		Message:   message,
		UserID:    s.currentHighestBidder,
		Timestamp: s.Clock.SendEvent(),
		Bid:       s.currentHighestBid,
	})
	s.currentHighestBid = 0
	s.currentHighestBidder = 0
}
