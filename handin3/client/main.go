package main

import (
	"context"
	"fmt"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

//

func main() {

	connection, error := grpc.Dial("localhost:1337", grpc.WithInsecure)

	if error != nil {
		fmt.Println("User connection failed")
	}

	client := pb.NewChatServiceClient(connection)

	client.Join(context.Background())

}
