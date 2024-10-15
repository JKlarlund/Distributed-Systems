package main

import (
	"flag"
	"fmt"
	"net"
)

var port *int = flag.Int("Port", 1337, "Server Port")

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))

}
