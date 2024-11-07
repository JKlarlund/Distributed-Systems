package handin4

import (
	"context"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
)

//Hvad vil vi gerne?
//1. Initialiser en node. Hvis den modtager et request, skal den håndtere det.

type Request struct {
	NodeID    int32
	Timestamp int32
}

type Data struct {
	clock        LClock
	requestQueue []Request
}

type NodeServer struct {
	protobufs.UnimplementedConsensusServer
	data *Data
}

func NewNodeServer(data *Data) *NodeServer {
	return &NodeServer{
		data: data,
	}
}

func main() {
	//To handle connecting a new node.
	var nodeID int32 = 1337
	nodeData := &Data{
		clock: LClock{id: nodeID, Time: 0},
	}

}

func (s *NodeServer) requestAccess(ctx context.Context, req *protobufs.AccessRequest) {

}

func requestCS(nodeData Data) {
	nodeData.clock.Step()
	//Lav et request = {tid, id}
	sendRequest(request)
	//Vent indtil antallet af replies er n-1.

}
