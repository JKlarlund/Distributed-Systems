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
	"strconv"
	"strings"
	"syscall"
)

type User struct {
	ID    int32
	Clock *chat.LClock
}

var (
	user    User
	logName string      = "Client-" + strconv.Itoa(os.Getpid())
	logger  *log.Logger = chat.InitLogger(logName)
)

func main() {
	conn, err := grpc.DialContext(context.Background(), "localhost:1337", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("User connection failed: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	chat.WriteToLog(logger, "Trying to connect user", 0, -1) //Time 0 for newly created client, user ID not given yet.
	client := pb.NewChatServiceClient(conn)

	fmt.Println("Connecting...")

	response, err := client.Join(context.Background(), &emptypb.Empty{})
	if err != nil {
		logger.Fatalf("User has failed to join the chat")

	}
	stream, err := client.ChatStream(context.Background())
	if err == nil {
		fmt.Println("Connection established as user")
		chat.WriteToLog(logger, "Connection has been established", 1, response.UserID) //Time 0 for newly created client, user ID not given yet.
	}

	chat.HandleFatalError(err)
	user = User{ID: response.UserID, Clock: chat.InitializeLClock(response.UserID, 0)}
	updatedClock := user.Clock.SendEvent()
	stream.Send(&pb.Message{UserID: user.ID, Timestamp: updatedClock, Body: "Connection has been established"})
	chat.WriteToLog(logger, "User has sent message", user.Clock.Time, user.ID)

	go listen(stream)
	go readInput(stream)

	<-sigs
	updatedClock = user.Clock.SendEvent()
	chat.WriteToLog(logger, "Requesting to leave", updatedClock, user.ID)
	msg, err := client.Leave(context.Background(), &pb.LeaveRequest{UserID: user.ID})
	chat.HandleFatalError(err)

	user.Clock.ReceiveEvent(msg.Timestamp)
	chat.WriteToLog(logger, "Connection to server has been closed", user.Clock.Time, user.ID)
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
			logger.Printf("Error while receiving message: %v", err)
			return
		}

		// Process the incoming message
		if in != nil {
			user.Clock.ReceiveEvent(in.Timestamp)
			logMessage := fmt.Sprintf("Received message: \"%v\" from: user %d", in.Body, in.UserID)
			chat.WriteToLog(logger, logMessage, user.Clock.Time, user.ID)
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
		message = strings.TrimSpace(message)
		if len(message) > 128 {
			fmt.Println("\033[1;31mMessage could not be sent since the length of the message cannot exceed 128 characters\u001B[0m")
			continue
		}
		user.Clock.SendEvent()
		err = stream.Send(&pb.Message{UserID: user.ID, Timestamp: user.Clock.Time, Body: message})
		chat.HandleFatalError(err)
	}
}
