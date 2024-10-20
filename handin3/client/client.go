package main

import (
	"bufio"
	"context"
	"fmt"
	chat "github.com/JKlarlund/Distributed-Systems/handin3"
	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type User struct {
	ID    int32
	Clock *chat.LClock
}

var user User

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User connection failed: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	client := pb.NewChatServiceClient(conn)

	fmt.Println("Connecting...")
	response, err := client.Join(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	stream, err := client.ChatStream(context.Background())
	if err == nil {
		fmt.Println("Connection established as user")
	}
	chat.HandleFatalError(err)
	user = User{ID: response.UserID, Clock: chat.InitializeLClock(response.UserID, 0)}
	stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: "Connection has been established"})

	go listen(stream)

	chat.HandleFatalError(err)

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
			// Check for server message
			if in.UserID == 0 {
				fmt.Printf("\033[1;34m[Server] %s\033[0m\n", in.Body)
			} else {
				fmt.Printf("User %v: %s\n", in.UserID, in.Body)
			}

		}
	}
}

func readInput(stream pb.ChatService_ChatStreamClient) {
	for {
		reader := bufio.NewReader(os.Stdin)
		message, err := reader.ReadString('\n')

		if len(message) > 128 {
			fmt.Println("\033[1;31mMessage could not be sent since the length of the message cannot exceed 128 characters\u001B[0m")
			continue
		}
		err = stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: message})
		chat.HandleFatalError(err)
	}
}
