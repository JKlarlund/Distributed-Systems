package main

import (
	"context"
	"flag"
	"fmt"
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
	Connection pb.ChatServiceClient
	Clock      *chat.LClock
}

var (
	users = make(map[int32]*User)
	mutex sync.Mutex
)

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	mutex.Lock()
	defer mutex.Unlock()

	nextUserID++

	s.Clock.Step()

	newUser := &User{
		userID: nextUserID,
		Clock:  chat.InitializeLClock(nextUserID, s.Clock.Time),
	}
	users[nextUserID] = newUser

	fmt.Printf("Participant %d joined chitty-chat at lamport time %v.\n", nextUserID, newUser.Clock.Time)

	return &pb.JoinResponse{
		Message: "User joined successfully",
		UserID:  nextUserID,
	}, nil
}

func (s *Server) PublishMessage(joinContext context.Context, message *pb.Message) (*pb.Ack, error) {
	for _, conn := range users {
		conn.Connection.ReceiveMessage(context.Background(), message)
	}

	return &pb.Ack{Message: "Success"}, nil

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
