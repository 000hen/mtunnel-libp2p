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

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

func runHost(host host.Host, networkType string, forwardPort int) {
	ctx, cancel := context.WithCancel(context.Background())

	session := NewSessionManager(host.Network())
	host.Network().Notify(&ConnListener{
		sm: session,
	})

	idht, err := setupDHT(ctx, host, true)
	if err != nil {
		cancel()
		_ = host.Close()
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to create DHT: %v", err),
		})
		log.Fatalf("Failed to create DHT: %v", err)
	}

	log.Println("Waiting for network stabilization...")
	time.Sleep(20 * time.Second)

	log.Println("Listening on addresses:")
	for _, addr := range host.Addrs() {
		log.Println(" - ", addr.String())
	}

	token := ConnToken{
		Network: networkType,
		ID:      host.ID(),
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
		if err := idht.Close(); err != nil {
			log.Printf("Error closing DHT: %v", err)
		}
		if err := host.Close(); err != nil {
			log.Printf("Error closing libp2p host: %v", err)
		}
		os.Exit(exitCode)
	}
	go handleIOAction(session)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	host.SetStreamHandler("/mtunnel/1.0.0", func(s network.Stream) {
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
