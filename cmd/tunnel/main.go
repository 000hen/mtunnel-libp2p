package main

import (
	"flag"
	"net"
)

type ConnectionToken struct {
	Fingerprint [32]byte
	Network     string
	Host        net.IP
	Port        int
}

func main() {
	port := flag.Int("port", 0, "Port to forward, in client mode this is the local port to connect to")
	network := flag.String("network", "tcp", "Network type for local connection: tcp or udp")
	token := flag.String("token", "", "Connection token for client mode")

	flag.Parse()

	h := initializePeer()
	if *token == "" {
		runHost(h, *network, *port)
	} else {
		runClient(h, *token, *port)
	}
}
