package main

import (
	"context"
	"log"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
)

func setupDHT(ctx context.Context, h host.Host, asServer bool) (*dht.IpfsDHT, error) {
	mode := dht.ModeClient
	if asServer {
		mode = dht.ModeAutoServer
	}

	dhtInstance, err := dht.New(ctx, h, dht.Mode(mode))
	if err != nil {
		return nil, err
	}

	if err := dhtInstance.Bootstrap(ctx); err != nil {
		_ = dhtInstance.Close()
		return nil, err
	}

	if err := connectToBootstrapPeers(ctx, h); err != nil {
		log.Printf("Bootstrap peer connection failed: %v", err)
	}

	return dhtInstance, nil
}

func connectToBootstrapPeers(ctx context.Context, h host.Host) error {
	for _, addr := range dht.GetDefaultBootstrapPeerAddrInfos() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := h.Connect(dialCtx, addr)
		cancel()

		if err != nil {
			log.Printf("Failed to connect to bootstrap peer %s: %v", addr.ID, err)
		} else {
			log.Printf("Connected to bootstrap peer: %s", addr.ID)
		}
	}

	return nil
}
