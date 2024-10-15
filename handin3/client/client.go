package main

import (
	"context"
	"fmt"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

//

func ReceiveMessage(messageContext context.Context, msg *pb.Message) (*pb.Ack, error) {

	fmt.Printf("User %d has sent message %s at timestamp %d", msg.UserID, msg.Body, msg.Timestamp)

	return &pb.Ack{Message: "success"}, nil
}

func main() {

	connection, error := grpc.NewClient("localhost:1337")

	if error != nil {
		fmt.Println("User connection failed")
	}

	client := pb.NewChatServiceClient(connection)

	response, joinError := client.Join(context.Background(), &pb.JoinRequest{UserID: 1})

	if joinError != nil {
		fmt.Println("Client couldn't connect, returning...")
		return
	}

	fmt.Printf("Success! You are user %d.\n", response.UserID)

	var input string
	_, err := fmt.Scan(&input)

	if err != nil {
		return
	}

	client.PublishMessage(context.Background(), &pb.Message{UserID: response.UserID, Timestamp: 1, Body: input})

}
