package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"
	"google.golang.org/grpc"
)

func ReceiveMessage(messageContext context.Context, msg *pb.Message) (*pb.Ack, error) {

	fmt.Printf("User %d has sent message %s at timestamp %d", msg.UserID, msg.Body, msg.Timestamp)

	return &pb.Ack{Message: "success"}, nil
}

func main() {
	// Set up a connection to the server with context timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User connection failed: %v", err)
	}
	defer conn.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	client := pb.NewChatServiceClient(conn)

	// Send the JoinRequest to the server.
	//ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	response, err := client.Join(context.Background())
	chat.HandleFatalError(err)

	user = User{ID: response.UserID, Clock: chat.InitializeLClock(response.UserID, 0)}

	fmt.Printf("Success! You are user %d.\n", user.ID)

	stream, err := client.ChatStream(context.Background())
	chat.HandleFatalError(err)

	go listen(stream)

	for {
		var input string
		_, err = fmt.Scan(&input)
		if err != nil {
			return
		}

		stream.Send(&pb.Message{UserID: 100, Timestamp: 1, Body: input})
	}

	<-sigs

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
