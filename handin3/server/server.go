package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

var nextUserID int32 = 0

type Server struct {
	pb.UnimplementedChatServiceServer
}

type User struct {
	userID     int32
	Connection pb.ChatServiceClient
}

var (
	users = make(map[int32]*User)
	mutex sync.Mutex
)

func (s *Server) Join(joinContext context.Context) {
	nextUserID++
	newUser := &User{userID: nextUserID}
	users[nextUserID] = newUser

}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	pb.RegisterChatServiceServer(server, &Server{})

	log.Printf("Server is listening on port %d...", *port)

	server.Serve(listener)

}
