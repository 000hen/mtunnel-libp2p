package main

import (
	"context"
	"errors"
	"log"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
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

func findPeerInDHT(ctx context.Context, dhtInstance *dht.IpfsDHT, peerID peer.ID) peer.AddrInfo {
	info, findErr := dhtInstance.FindPeer(ctx, peerID)
	if findErr == nil {
		log.Printf("Discovered peer %s via DHT with %d addresses", info.ID, len(info.Addrs))
		log.Println("Addresses found via DHT:")
		for _, a := range info.Addrs {
			log.Println(" -", a.String())
		}
	} else if errors.Is(findErr, routing.ErrNotFound) {
		log.Fatalf("Peer %s not yet available in DHT", peerID)
	} else {
		log.Fatalf("DHT lookup for peer %s failed: %v", peerID, findErr)
	}

	return info
}
