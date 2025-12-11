package main

import (
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

func initializePeer() host.Host {
	h, err := libp2p.New(
		libp2p.EnableHolePunching(),
		libp2p.EnableAutoRelayWithStaticRelays(dht.GetDefaultBootstrapPeerAddrInfos()),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableNATService(),
		// libp2p.ForceReachabilityPrivate(),
	)
	if err != nil {
		sendOutputAction(OutputAction{
			Action: ERROR,
			Error:  fmt.Sprintf("Failed to create libp2p host: %v", err),
		})
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	log.Println("Libp2p host created with ID:", h.ID())

	return h
}

func connectToPeer(ctx context.Context, host host.Host, peer peer.AddrInfo) error {
	if err := host.Connect(ctx, peer); err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peer.ID, err)
	}

	log.Println("Successfully connected to peer:", peer.ID)
	log.Println("Connect with the address(es):")
	for _, addr := range host.Network().ConnsToPeer(peer.ID) {
		log.Println(" -", addr.RemoteMultiaddr().String())
	}

	return nil
}
