package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"net"
	"sync"

	chat "github.com/JKlarlund/Distributed-Systems/handin3"
	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"
	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

var nextUserID int32 = 0

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
	updatedTime := s.Clock.SendEvent()

	joinMessage := fmt.Sprintf("Participant %d joined chitty-chat at lamport time %v.\n", nextUserID, updatedTime)
	joinAnnouncement := &pb.Message{UserID: nextUserID, Timestamp: updatedTime, Body: joinMessage}

	_, err := s.PublishMessage(context.Background(), joinAnnouncement)
	if err != nil {
		log.Printf("Failed to publish join message")
	}
	nextUserID++
	return &pb.JoinResponse{
		Message: "User joined successfully",
		UserID:  nextUserID - 1,
	}, nil
}

func (s *Server) PublishMessage(joinContext context.Context, message *pb.Message) (*pb.Ack, error) {
	for _, conn := range users {
		if conn.userID != message.UserID {
			err := conn.Connection.Send(message)
			if err != nil {
				log.Printf("Failed to send message to a user.\n")
			}
		}
	}

	return &pb.Ack{Message: "Success"}, nil

}

func (s *Server) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	for {

		msg, err := stream.Recv()
		if msg == nil {
			return fmt.Errorf("received nil message")
		}
		chat.HandleError(err)

		fmt.Printf("user id is: %d", msg.UserID)
		sender := users[msg.UserID]
		if sender.Connection == nil {
			sender.Connection = stream
		}

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

	pb.RegisterChatServiceServer(server, chatServer)

	log.Printf("Server is listening on port %d...", *port)

	server.Serve(listener)
}
