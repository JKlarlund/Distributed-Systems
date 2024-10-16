package main

import (
	"context"
	"fmt"
	chat "github.com/JKlarlund/Distributed-Systems/handin3"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"time"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"
	"google.golang.org/grpc"
)

type User struct {
	ID    int32
	Clock *chat.LClock
}

var user User

func main() {
	// Set up a connection to the server with context timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User connection failed: %v", err)
	}
	defer conn.Close()

	//sigs := make(chan os.Signal, 1)
	//signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	client := pb.NewChatServiceClient(conn)

	// Send the JoinRequest to the server.
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	response, err := client.Join(ctx, &emptypb.Empty{})
	chat.HandleFatalError(err)

	user = User{ID: response.UserID, Clock: chat.InitializeLClock(response.UserID, 0)}

	fmt.Printf("Success! You are user %d.\n", user.ID)

	stream, err := client.ChatStream(ctx)
	chat.HandleFatalError(err)

	go listen(stream)
	readInput(stream)

	//<-sigs

	stream.CloseSend()
	fmt.Println("You have now exited the chat application")
}

func listen(stream pb.ChatService_ChatStreamClient) {
	for {
		in, err := stream.Recv()
		chat.HandleFatalError(err)
		user.Clock.ReceiveEvent(in.Timestamp)
		fmt.Println(in)
	}
}

func readInput(stream pb.ChatService_ChatStreamClient) {
	for {
		var input string
		_, err := fmt.Scan(&input)
		chat.HandleError(err)

		err = stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: input})
		chat.HandleFatalError(err)
	}
}
