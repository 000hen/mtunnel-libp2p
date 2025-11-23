package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
)

func runClient(host host.Host, token string, localPort int) {
	var decodedToken ConnToken
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		log.Fatalf("Failed to decode token: %v", err)
	}

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&decodedToken); err != nil {
		log.Fatalf("Failed to decode token data: %v", err)
	}

	dhtCtx, dhtCancel := context.WithCancel(context.Background())
	clientDHT, err := setupDHT(dhtCtx, host, false)
	if err != nil {
		dhtCancel()
		_ = host.Close()
		log.Fatalf("Failed to bootstrap DHT: %v", err)
	}

	log.Println("Routing table size:", clientDHT.RoutingTable().Size())

	info := findPeerInDHT(dhtCtx, clientDHT, decodedToken.ID)
	dhtCancel()

	cleanupTransport := func() {
		if err := clientDHT.Close(); err != nil {
			log.Printf("Error closing DHT: %v", err)
		}
		if err := host.Close(); err != nil {
			log.Printf("Error closing libp2p host: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := connectToPeer(ctx, host, info); err != nil {
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to connect to peer %s: %v", decodedToken.ID, err),
		})
		log.Fatalf("Failed to connect to peer %s: %v", decodedToken.ID, err)
	}

	listen, err := net.Listen(decodedToken.Network, fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		cleanupTransport()
		log.Fatalf("Failed to listen on local port %d: %v", localPort, err)
	}
	defer listen.Close()

	log.Printf("Listening for local connections on %s\n", listen.Addr().String())

	port := listen.Addr().(*net.TCPAddr).Port
	sendOutputAction(OutputAction{
		Action:    CONNECTED,
		SessionId: host.ID(),
		Port:      port,
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		cause := context.Cause(ctx)
		if cause == nil {
			cause = context.Canceled
		}
		log.Printf("Connection lost: %v", cause)
		log.Println("Stopping local listener...")
		listen.Close()
		cleanupTransport()
		cancel()
	}()

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		log.Println("Initiating graceful shutdown...")
		cancel()
		os.Exit(0)
	}()

	for {
		localConn, err := listen.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Local listener closed, exiting...")
				cleanupTransport()
				return
			}

			log.Printf("Failed to accept local connection: %v", err)
			continue
		}

		streamCtx, streamCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer streamCancel()

		s, err := host.NewStream(streamCtx, decodedToken.ID, "/mtunnel/1.0.0")
		if err != nil {
			log.Printf("Failed to open stream to peer %s: %v", decodedToken.ID, err)
			localConn.Close()
			continue
		}

		log.Printf("Opened stream to peer %s\n", decodedToken.ID)
		pipe(s, localConn)
		log.Printf("Closed stream to peer %s\n", decodedToken.ID)
	}
}
