package chat

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

type chatServiceServer struct {
	pb.UnimplementedRouteGuideServer
	savedFeatures []*pb.Feature // read-only after initialized

	mu         sync.Mutex // protects routeNotes
	routeNotes map[string][]*pb.RouteNote
}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	pb.RegisterChatServiceServer(server, newServer())

	server.Serve(listener)

}

func newServer() *chatServiceServer {
	s := &chatServiceServer{routeNotes: make(map[string][]*pb.RouteNote)}
	return s
}
