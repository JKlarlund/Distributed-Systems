package server

import (
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin5"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin5/protobufs"
)

type Server struct {
	pb.UnimplementedAuctionServiceServer
	Clock *Clock.LClock
}
