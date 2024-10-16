package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"

	chat "github.com/JKlarlund/Distributed-Systems/handin3"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

var nextUserID int32 = 1
var serverUserID int32 = 0

type Server struct {
	pb.UnimplementedChatServiceServer
	Clock *chat.LClock
}

type User struct {
	userID     int32
	Connection pb.ChatService_ChatStreamServer
}

var (
	users = make(map[int32]*User)
	mutex sync.Mutex
)

func (s *Server) Join(ctx context.Context, empty *emptypb.Empty) (*pb.JoinResponse, error) {
	newUser := &User{
		userID: nextUserID,
	}
	users[nextUserID] = newUser

	return &pb.JoinResponse{
		Message: "Ack",
		UserID:  nextUserID,
	}, nil
}

func (s *Server) PublishMessage(joinContext context.Context, message *pb.Message) (*pb.Ack, error) {
	for _, user := range users {
		if user.userID == message.UserID || user.userID == serverUserID {
			continue
		}
		err := user.Connection.Send(message)
		if err != nil {
			log.Printf("Failed to send message to a user.\n")
		}

	}

	return &pb.Ack{Message: "Success"}, nil

}

func (s *Server) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	// Set up the user's connection immediately
	msg, err := stream.Recv()
	if msg == nil {
		return fmt.Errorf("received nil message")
	}
	chat.HandleError(err)

	sender := users[msg.UserID]
	if sender.Connection == nil {
		sender.Connection = stream
	}

	updatedTime := s.Clock.SendEvent()

	joinMessage := fmt.Sprintf("Participant %d joined chitty-chat at lamport time %v.\n", nextUserID, updatedTime)

	s.PublishMessage(context.Background(), &pb.Message{UserID: serverUserID, Timestamp: updatedTime, Body: joinMessage})

	nextUserID++
	// Now enter the loop to listen for new messages
	for {
		msg, err := stream.Recv()
		if msg == nil {
			return fmt.Errorf("received nil message")
		}
		chat.HandleError(err)

		_, err = s.PublishMessage(context.Background(), msg)
		chat.HandleFatalError(err)
	}
}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	chatServer := &Server{
		Clock: chat.InitializeLClock(0, 0),
	}

	// We register the server as user 0 for admin purposes
	users[0] = &User{
		userID: 0,
	}

	pb.RegisterChatServiceServer(server, chatServer)

	log.Printf("Server is listening on port %d...", *port)

	server.Serve(listener)
}
