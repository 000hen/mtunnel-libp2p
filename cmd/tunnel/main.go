package main

import (
	"flag"
	"log"
)

func main() {
	port := flag.Int("port", 0, "Port to forward, in client mode this is the local port to connect to")
	network := flag.String("network", "tcp", "Network type for local connection: tcp or udp")
	token := flag.String("token", "", "Connection token for client mode")

	flag.Parse()

	if err := validateNetworkType(*network); err != nil {
		log.Fatalf("Invalid network type: %v", err)
	}

	if *port < 0 {
		log.Fatalf("Port must be a non-negative integer")
	}

	if *token == "" && *port == 0 {
		log.Fatalf("Host mode requires a non-zero port to forward")
	}

	h := initializePeer()
	if *token == "" {
		runHost(h, *network, *port)
	} else {
		runClient(h, *token, *port)
	}
}
