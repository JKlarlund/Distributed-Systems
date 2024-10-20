package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"

	chat "github.com/JKlarlund/Distributed-Systems/handin3"

	pb "github.com/JKlarlund/Distributed-Systems/handin3/protobufs"

	"google.golang.org/grpc"
)

var port *int = flag.Int("Port", 1337, "Server Port")

var nextUserID int32 = 1
var serverUserID int32 = 0

type Server struct {
	pb.UnimplementedChatServiceServer
	Clock *chat.LClock
}

type User struct {
	userID     int32
	Connection pb.ChatService_ChatStreamServer
}

var (
	users  = make(map[int32]*User)
	mutex  sync.Mutex
	logger = chat.InitLogger("server")
)

func (s *Server) Join(ctx context.Context, empty *emptypb.Empty) (*pb.JoinResponse, error) {
	newUser := &User{
		userID: nextUserID,
	}
	users[nextUserID] = newUser

	updatedClock := s.Clock.SendEvent()
	chat.WriteToLog(logger, "has joined the chat", updatedClock, nextUserID)

	return &pb.JoinResponse{
		Message: "Ack",
		UserID:  nextUserID,
	}, nil
}

func (s *Server) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	_, exists := users[req.UserID]
	updatedClock := s.Clock.SendEvent()
	if !exists {
		return &pb.LeaveResponse{Message: "Users not found", Timestamp: updatedClock}, nil
	}
	delete(users, req.UserID)
	chat.WriteToLog(logger, "has left the chat", updatedClock, req.UserID)
	leaveMessage := fmt.Sprintf("Participant %d left Chitty-Chat at Lamport time %v", req.UserID, updatedClock)
	s.PublishMessage(ctx, &pb.Message{UserID: serverUserID, Timestamp: updatedClock, Body: leaveMessage})
	return &pb.LeaveResponse{Message: "Success", Timestamp: updatedClock}, nil
}

func (s *Server) PublishMessage(Context context.Context, message *pb.Message) (*pb.Ack, error) {
	updatedClock := s.Clock.SendEvent()
	logmsg := fmt.Sprintf("Broadcasting message: \"%s\"", message.Body)
	chat.WriteToLog(logger, logmsg, updatedClock, message.UserID)
	for _, user := range users {
		if user.userID == message.UserID || user.userID == serverUserID {
			continue
		}
		message.Timestamp = updatedClock
		err := user.Connection.Send(message)
		if err != nil {
			chat.WriteToLog(logger, "Failed to write to user", updatedClock, nextUserID)
		}

	}

	return &pb.Ack{Message: "Success"}, nil

}

func (s *Server) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	// Set up the user's connection immediately
	msg, err := stream.Recv()
	if msg == nil {
		return fmt.Errorf("received nil message")
	}
	chat.HandleError(err)

	sender := users[msg.UserID]
	if sender.Connection == nil {
		sender.Connection = stream
	}
	s.Clock.ReceiveEvent(msg.Timestamp)
	chat.WriteToLog(logger, "Client ready to stream to server", s.Clock.Time, nextUserID)

	updatedClock := s.Clock.SendEvent()
	joinMessage := fmt.Sprintf("Participant %d joined chitty-chat at Lamport time %v", nextUserID, updatedClock)
	logMessage := fmt.Sprintf("Participant %d joined chitty-chat", nextUserID)
	chat.WriteToLog(logger, logMessage, updatedClock, serverUserID)

	s.PublishMessage(context.Background(), &pb.Message{UserID: serverUserID, Timestamp: updatedClock, Body: joinMessage})

	nextUserID++
	// Now enter the loop to listen for new messages
	for {
		msg, err := stream.Recv()
		if msg == nil {
			return fmt.Errorf("received nil message")
		}
		if msg.Body == "" {
			return nil
		}
		chat.HandleError(err)

		s.Clock.ReceiveEvent(msg.Timestamp)
		chat.WriteToLog(logger, "Server received a message from client", s.Clock.Time, msg.UserID)

		_, err = s.PublishMessage(context.Background(), msg)
		chat.HandleFatalError(err)
	}
}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()

	chatServer := &Server{
		Clock: chat.InitializeLClock(0, 0),
	}

	// We register the server as user 0 for admin purposes
	users[0] = &User{
		userID: 0,
	}

	pb.RegisterChatServiceServer(server, chatServer)
	logMessage := fmt.Sprintf("Server is listening on port %d...", *port)
	chat.WriteToLog(logger, logMessage, chatServer.Clock.Time, 0)
	chat.ClearClientLogs()
	server.Serve(listener)
}
