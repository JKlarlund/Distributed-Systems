package main

import (
	"context"
	"fmt"
	"log"
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

	client := pb.NewChatServiceClient(conn)

	// Create a JoinRequest message.
	req := &pb.JoinRequest{
		UserID: 1,
	}

	// Send the JoinRequest to the server.
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := client.Join(ctx, req)
	if err != nil {
		log.Fatalf("Client couldn't connect, returning: %v", err)
	}

	fmt.Printf("Success! You are user %d.\n", res.UserID)

	stream, err := client.ChatStream(context.Background())

	if err != nil {
		fmt.Println(err)
		return
	}

	go listen(stream)

	var input string
	_, err = fmt.Scan(&input)
	if err != nil {
		return
	}

	client.PublishMessage(context.Background(), &pb.Message{UserID: res.UserID, Timestamp: 1, Body: input})
}

func listen(stream pb.ChatService_ChatStreamClient) {
	for {
		in, err := stream.Recv()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(in)
	}
}
