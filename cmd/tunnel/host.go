package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/network"
)

func runHost(networkType string, forwardPort int) {
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
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to create libp2p host: %v", err),
		})
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	session := NewSessionManager(h.Network())

	h.Network().Notify(&ConnListener{
		sm: session,
	})

	idht, err := setupDHT(ctx, h, true)
	if err != nil {
		cancel()
		_ = h.Close()
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to create DHT: %v", err),
		})
		log.Fatalf("Failed to create DHT: %v", err)
	}

	log.Println("Waiting for network stabilization...")
	time.Sleep(20 * time.Second)

	log.Println("Libp2p host created with ID:", h.ID())
	log.Println("Listening on addresses:")
	for _, addr := range h.Addrs() {
		log.Println(" - ", addr.String())
	}

	token := ConnToken{
		Network: networkType,
		ID:      h.ID(),
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(token); err != nil {
		log.Fatalf("Failed to encode token: %v", err)
	}

	encodedToken := base64.StdEncoding.EncodeToString(buf.Bytes())

	log.Println("Connection Token:")
	log.Println("-------------")
	log.Println(encodedToken)
	log.Println("-------------")

	sendOutputAction(OutputAction{
		Action: TOKEN,
		Token:  encodedToken,
	})

	cleanup := func(exitCode int) {
		cancel()
		session.ForceCloseAllSessions()
		if idht != nil {
			if err := idht.Close(); err != nil {
				log.Printf("Error closing DHT: %v", err)
			}
			idht = nil
		}
		if err := h.Close(); err != nil {
			log.Printf("Error closing libp2p host: %v", err)
		}
		os.Exit(exitCode)
	}
	go handleIOAction(session)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	h.SetStreamHandler("/mtunnel/1.0.0", func(s network.Stream) {
		session.AddSession(s.Conn().RemotePeer(), s.Conn())
		handleStream(s, networkType, forwardPort)
		session.RemoveSession(s.Conn().RemotePeer())
	})

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Initiating graceful shutdown...")
	cleanup(0)
}

func handleStream(s network.Stream, network string, forwardPort int) {
	log.Println("New stream opened from:", s.Conn().RemotePeer())
	defer s.Close()

	addr := fmt.Sprintf("localhost:%d", forwardPort)
	localConn, err := net.Dial(network, addr)
	if err != nil {
		log.Printf("Failed to connect to local service on %s: %v", addr, err)
		s.Close()
		return
	}
	defer localConn.Close()

	log.Printf("Connected to local service on %s\n", addr)

	pipe(s, localConn)
	log.Println("Stream closed for peer:", s.Conn().RemotePeer())
}
