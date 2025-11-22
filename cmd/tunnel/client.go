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

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/routing"
)

func runClient(token string, localPort int) {
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

	ctx, cancel := context.WithCancel(context.Background())
	h, err := libp2p.New(
		libp2p.EnableHolePunching(),
		libp2p.EnableAutoRelayWithStaticRelays(dht.GetDefaultBootstrapPeerAddrInfos()),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableNATService(),
		libp2p.ForceReachabilityPrivate(),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	log.Println("Libp2p host created with ID:", h.ID())

	clientDHT, err := setupDHT(ctx, h, false)
	if err != nil {
		cancel()
		_ = h.Close()
		log.Fatalf("Failed to bootstrap DHT: %v", err)
	}

	log.Println("Routing table size:", clientDHT.RoutingTable().Size())

	cleanupTransport := func() {
		cancel()
		if clientDHT != nil {
			if err := clientDHT.Close(); err != nil {
				log.Printf("Error closing DHT: %v", err)
			}
			clientDHT = nil
		}
		if h != nil {
			if err := h.Close(); err != nil {
				log.Printf("Error closing libp2p host: %v", err)
			}
			h = nil
		}
	}
	defer cleanupTransport()

	info, findErr := clientDHT.FindPeer(ctx, decodedToken.ID)
	if findErr == nil {
		log.Printf("Discovered peer %s via DHT with %d addresses", info.ID, len(info.Addrs))
		log.Println("Addresses found via DHT:")
		for _, a := range info.Addrs {
			log.Println(" -", a.String())
		}
	} else if errors.Is(findErr, routing.ErrNotFound) {
		log.Fatalf("Peer %s not yet available in DHT", decodedToken.ID)
	} else {
		log.Fatalf("DHT lookup for peer %s failed: %v", decodedToken.ID, findErr)
	}

	if err := h.Connect(ctx, info); err != nil {
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to connect to peer %s: %v", decodedToken.ID, err),
		})
		log.Fatalf("Failed to connect to peer %s: %v", decodedToken.ID, err)
	}

	log.Println("Successfully connected to peer:", decodedToken.ID)
	log.Println("Connect with the address(es):")
	for _, addr := range h.Network().ConnsToPeer(decodedToken.ID) {
		log.Println(" -", addr.RemoteMultiaddr().String())
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
		SessionId: h.ID(),
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
	}()

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		log.Println("Initiating graceful shutdown...")
		listen.Close()
		cleanupTransport()
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

		s, err := h.NewStream(streamCtx, decodedToken.ID, "/mtunnel/1.0.0")
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
