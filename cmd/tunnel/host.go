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
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

func runHost(host host.Host, networkType string, forwardPort int) {
	ctx, cancel := context.WithCancel(context.Background())

	session := NewSessionManager(host.Network())

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
	time.Sleep(networkStabilizationDelay)

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

	var cleanupOnce sync.Once
	var wg sync.WaitGroup

	cleanup := func() {
		cleanupOnce.Do(func() {
			log.Println("Cleaning up...")
			cancel()
			session.ForceCloseAllSessions()
			wg.Wait()

			if err := idht.Close(); err != nil {
				log.Printf("Error closing DHT: %v", err)
			}
			if err := host.Close(); err != nil {
				log.Printf("Error closing libp2p host: %v", err)
			}
		})
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		handleIOAction(ctx, session)
	}()

	host.SetStreamHandler(protocolID, func(s network.Stream) {
		wg.Add(1)
		defer wg.Done()

		peer := s.Conn().RemotePeer()
		session.AddSession(peer, s.Conn())

		handleStream(s, networkType, forwardPort)

		session.RemoveSession(s.Conn().RemotePeer())
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Initiating graceful shutdown...")

	cleanup()

	log.Println("Shutdown completed.")
}

func handleStream(s network.Stream, network string, forwardPort int) {
	log.Println("New stream opened from:", s.Conn().RemotePeer())
	defer s.Close()

	addr := fmt.Sprintf("localhost:%d", forwardPort)
	localConn, err := net.DialTimeout(network, addr, defaultLocalDialTimeout)
	if err != nil {
		log.Printf("Failed to connect to local service on %s: %v", addr, err)
		return
	}
	defer localConn.Close()

	log.Printf("Connected to local service on %s", addr)

	pipe(s, localConn)
	log.Println("Stream closed for peer:", s.Conn().RemotePeer())
}
