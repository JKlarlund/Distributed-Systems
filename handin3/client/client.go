package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	chat "github.com/JKlarlund/Distributed-Systems/handin3"
	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type User struct {
	ID    int32
	Clock *chat.LClock
}

var user User

func main() {
	// Set up a connection to the server with context timeout.
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User connection failed: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	client := pb.NewChatServiceClient(conn)

	// Send the JoinRequest to the server.
	//ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	response, err := client.Join(context.Background(), &emptypb.Empty{})
	stream, err := client.ChatStream(context.Background())

	chat.HandleFatalError(err)
	user = User{ID: response.UserID, Clock: chat.InitializeLClock(response.UserID, 0)}

	stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: response.Message})

	fmt.Printf("Success! You are user %d.\n", user.ID)

	chat.HandleFatalError(err)
	go listen(stream)
	go readInput(stream)

	<-sigs

	stream.CloseSend()
	fmt.Println("You have now exited the chat application")
}

func listen(stream pb.ChatService_ChatStreamClient) {
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				fmt.Println("Server closed the stream.")
				return
			}
			log.Printf("Error while receiving message: %v", err)
			return
		}

		// Process the incoming message
		if in != nil {
			user.Clock.ReceiveEvent(in.Timestamp)
			fmt.Printf("Received message: %s\n", in.Body)
		}
	}
}

func readInput(stream pb.ChatService_ChatStreamClient) {
	for {
		reader := bufio.NewReader(os.Stdin)
		message, err := reader.ReadString('\n')
		err = stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: message})
		chat.HandleFatalError(err)
	}
}
