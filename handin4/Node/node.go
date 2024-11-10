package Node

import (
	"context"
	"fmt"
	"github.com/JKlarlund/Distributed-Systems/tree/main/handin4/Clock"
	pb "github.com/JKlarlund/Distributed-Systems/tree/main/handin4/protobufs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type NodeServer struct {
	pb.UnimplementedConsensusServer
	NodeID             int32
	Clock              *Clock.LClock
	RequestedTimestamp int32 // When node requested access to critical section. Should me initialized to MAX INT
	Mutex              sync.Mutex
	Clients            map[string]pb.ConsensusClient // Stores addresses of discovered nodes, excluding itself
	selfAddress        string                        // Store the current node's address
}

var nodeIsWaitingForAccess = false
var nodeIsInCriticalSection = false

func InitializeDiscovery(node *NodeServer) {
	ip, _ := getLocalIP()                                    // Gets the local IP address.
	node.selfAddress = fmt.Sprintf("%s:%d", ip, node.NodeID) //IP:Port.
	go node.broadcastNode(0)
	go node.broadcastNode(1)
	go node.broadcastNode(2)
	//Starts broadcasting that the node is there
	go node.handleDiscoveryEvent(0)
	go node.handleDiscoveryEvent(1)
	go node.handleDiscoveryEvent(2)

}

func (s *NodeServer) broadcastNode(id int) {
	addr := fmt.Sprintf("%s:%d", "255.255.255.255", 54321+id)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		fmt.Println("Something went wrong with broadcasting node")
	}
	defer conn.Close()
	// Broadcast the node's information (e.g., IP address and NodeID)
	for {
		message := fmt.Sprintf("NodeID:%d, Address:%s", s.NodeID, s.selfAddress)
		_, err := fmt.Fprintf(conn, message)
		if err != nil {
			log.Printf("Error broadcasting: %v", err)
			return
		}

		// Wait before broadcasting again (e.g., every 2 seconds)
		time.Sleep(time.Second)
	}
}

func (s *NodeServer) handleDiscoveryEvent(id int32) {
	address := fmt.Sprintf("255.255.255.255:%d", 54321+id)

	addr, err := net.ResolveUDPAddr("udp", address) // Listen on UDP port 54321
	if err != nil {
		log.Printf("Error resolving address: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("Error creating UDP listener: %v", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024) // Buffer for incoming data
	for {
		// Read data from the UDP connection
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		// Process the received message
		message := string(buf[:n])
		//log.Printf("Received message from %s: %s", remoteAddr, message)

		// Extract the NodeID and Address (assuming the message format is consistent)
		var nodeID int
		var nodeAddress string
		_, err = fmt.Sscanf(message, "NodeID:%d, Address:%s", &nodeID, &nodeAddress)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Avoid responding to itself
		if nodeAddress != s.selfAddress {
			// Handle the discovery of the other node
			s.initializeConnection(nodeAddress)
		}
	}
}

// Inserts a k,v pair from our map
func (s *NodeServer) initializeConnection(target string) {
	_, ok := s.Clients[target]
	if !ok {
		conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		fmt.Printf("Node id is %d", s.NodeID)
		fmt.Println(string(s.NodeID) + ": " + "Adding address " + target)
		if err != nil {
			fmt.Println("Error: " + err.Error())
		}

		s.Clients[target] = pb.NewConsensusClient(conn)

	}
}

// Deletes a k,v pair from our map
func (s *NodeServer) severConnection(target string) {
	_, ok := s.Clients[target]
	if ok {
		delete(s.Clients, target)
	}
}

func (s *NodeServer) RequestAccessToCriticalSection() {
	nodeIsWaitingForAccess = true

	s.RequestedTimestamp = s.Clock.SendEvent() // Incrementing timestamp once before saving it
	var wg sync.WaitGroup
	wg.Add(len(s.Clients))

	request := &pb.AccessRequest{
		NodeID:    s.NodeID,
		Timestamp: s.RequestedTimestamp,
		Address:   s.selfAddress,
	}

	//We need to wait for clients to have n-1 approvals. Then go write something.
	for _, client := range s.Clients {
		go s.requestSingleAccess(&client, request, &wg)
	}

	wg.Wait()

	s.emulateCriticalSection()

	nodeIsWaitingForAccess = false

}

func (s *NodeServer) emulateCriticalSection() {
	nodeIsInCriticalSection = true
	log.Println(fmt.Sprintf("Node %d is in the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	time.Sleep(2 * time.Second)
	log.Println(fmt.Sprintf("Node %d is leaving the critical section at lamport time %d", s.NodeID, s.Clock.Time))
	nodeIsInCriticalSection = false
}

func (s *NodeServer) requestSingleAccess(client *pb.ConsensusClient, request *pb.AccessRequest, wg *sync.WaitGroup) {
	defer wg.Done()

	context, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.Clock.SendEvent() // Incrementing the lamport clock for each send event
	_, err := (*client).RequestAccess(context, request)
	if err != nil {
		log.Fatalf("requestSingleAccess: " + err.Error())
	}
}

// RequestAccess The purpose is to receive a request, decide if access should be granted to the requesting node, and
// send a response back to the requesting node.
func (s *NodeServer) RequestAccess(ctx context.Context, req *pb.AccessRequest) (*pb.AccessResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	log.Printf("Received access request from Node %d with timestamp %d\n", req.NodeID, req.Timestamp)

	// Updated and incremented local clock since this is a receive event
	s.Clock.ReceiveEvent(req.Timestamp)

	// Checking if access should be granted to the requesting node
	accessGranted := shouldHaveAccess(req.Timestamp, s.RequestedTimestamp)
	response := &pb.AccessResponse{
		NodeID:    s.NodeID,
		Timestamp: s.Clock.SendEvent(), // Incrementing the Lamport clock
		Access:    accessGranted,
	}
	for !accessGranted {
		time.Sleep(time.Second)
		if shouldHaveAccess(req.Timestamp, s.RequestedTimestamp) {
			return response, nil
		}
	}

	return response, nil
}

func shouldHaveAccess(requestTime int32, OwnRequestTime int32) bool {
	// Checking if the node is in the critical section
	if nodeIsInCriticalSection {
		return false
	}
	// Checking if the node requested access before this node
	if nodeIsWaitingForAccess && requestTime > OwnRequestTime {
		return false
	}
	return true
}

func getLocalIP() (string, error) {
	// Get all the network interfaces on the machine
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// Loop through all interfaces and get the first non-loopback IPv4 address
	for _, iface := range interfaces {
		// Skip loopback interfaces (127.0.0.1)
		if iface.Flags&net.FlagUp == 0 || strings.HasPrefix(iface.Name, "lo") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			// Check if it's an IPv4 address (ignoring IPv6 addresses)
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid IP address found")
}
