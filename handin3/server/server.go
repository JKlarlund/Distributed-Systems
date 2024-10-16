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
	Connection pb.ChatService_ChatStreamServer
	Clock      *chat.LClock
}

var (
	users = make(map[int32]*User)
	mutex sync.Mutex
)

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	mutex.Lock()
	defer mutex.Unlock()

	newUser := &User{
		userID: nextUserID,
		Clock:  chat.InitializeLClock(nextUserID, s.Clock.Time),
	}
	users[nextUserID] = newUser
	s.Clock.Step()
	fmt.Printf("Participant %d joined chitty-chat at lamport time %v.\n", nextUserID, newUser.Clock.Time)
	nextUserID++
	return &pb.JoinResponse{
		Message: "User joined successfully",
		UserID:  nextUserID - 1,
	}, nil
}

func (s *Server) PublishMessage(joinContext context.Context, message *pb.Message) (*pb.Ack, error) {
	for _, conn := range users {
		conn.Connection.Send(message)
	}

	return &pb.Ack{Message: "Success"}, nil

}

func (s *Server) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	for {

		msg, err := stream.Recv()
		if err != nil {
			fmt.Println(err)
			return err

		}

		sender := users[msg.UserID]
		if sender.Connection == nil {
			sender.Connection = stream
		}

		_, err = s.PublishMessage(context.Background(), msg)
		if err != nil {
			fmt.Println(err)
			return err
		}
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
