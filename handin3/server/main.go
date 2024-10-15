package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

type chatServiceServer struct {
	pb.UnimplementedChatServiceServer
}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	pb.RegisterChatServiceServer(server, &chatServiceServer{})

	log.Printf("Server is listening on port %d...", *port)

	server.Serve(listener)

}
