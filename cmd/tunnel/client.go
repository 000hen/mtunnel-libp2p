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
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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

	var cleanupOnce sync.Once
	cleanupTransport := func() {
		cleanupOnce.Do(func() {
			if err := clientDHT.Close(); err != nil {
				log.Printf("Error closing DHT: %v", err)
			}
			if err := host.Close(); err != nil {
				log.Printf("Error closing libp2p host: %v", err)
			}
		})
	}
	defer cleanupTransport()

	ctx, cancel := context.WithCancel(context.Background())
	if err := connectToPeer(ctx, host, info); err != nil {
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to connect to peer %s: %v", decodedToken.ID, err),
		})
		cleanupTransport()
		log.Fatalf("Failed to connect to peer %s: %v", decodedToken.ID, err)
	}

	listen, err := net.Listen(decodedToken.Network, fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		cleanupTransport()
		log.Fatalf("Failed to listen on local port %d: %v", localPort, err)
	}
	defer listen.Close()

	log.Printf("Listening for local connections on %s", listen.Addr().String())

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
	}()

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		log.Println("Initiating graceful shutdown...")
		cancel()
	}()

	for {
		localConn, err := listen.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Local listener closed, exiting...")
				return
			}

			log.Printf("Failed to accept local connection: %v", err)
			continue
		}

		go handleClientStream(ctx, host, decodedToken.ID, localConn)
	}
}

func handleClientStream(ctx context.Context, host host.Host, peer peer.ID, conn net.Conn) {
	streamCtx, streamCancel := context.WithTimeout(ctx, 30*time.Second)
	defer streamCancel()

	s, err := host.NewStream(streamCtx, peer, protocolID)
	if err != nil {
		log.Printf("Failed to open stream to peer %s: %v", peer, err)
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Error closing local connection: %v", closeErr)
		}
		return
	}
	defer s.Close()

	log.Printf("Opened stream to peer %s", peer)
	pipe(s, conn)
	log.Printf("Closed stream to peer %s", peer)
}
