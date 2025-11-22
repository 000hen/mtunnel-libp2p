package main

import (
	"github.com/libp2p/go-libp2p/core/network"
	multiaddr "github.com/multiformats/go-multiaddr"
)

type ConnListener struct {
	sm *SessionManager
}

func (c *ConnListener) Listen(network.Network, multiaddr.Multiaddr)      {}
func (c *ConnListener) ListenClose(network.Network, multiaddr.Multiaddr) {}

func (c *ConnListener) Connected(n network.Network, conn network.Conn) {
	// log.Println("New connection established with peer:", conn.RemotePeer())
}

func (c *ConnListener) Disconnected(n network.Network, conn network.Conn) {
	// log.Println("Connection closed with peer:", conn.RemotePeer())
}
