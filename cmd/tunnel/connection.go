package main

import (
	"log"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type remoteDisconnectWatcher struct {
	target peer.ID
	notify func(string)
}

func newRemoteDisconnectWatcher(target peer.ID, notify func(string)) *remoteDisconnectWatcher {
	return &remoteDisconnectWatcher{target: target, notify: notify}
}

func (w *remoteDisconnectWatcher) Listen(network.Network, ma.Multiaddr)         {}
func (w *remoteDisconnectWatcher) ListenClose(network.Network, ma.Multiaddr)    {}
func (w *remoteDisconnectWatcher) Connected(network.Network, network.Conn)      {}
func (w *remoteDisconnectWatcher) OpenedStream(network.Network, network.Stream) {}
func (w *remoteDisconnectWatcher) ClosedStream(network.Network, network.Stream) {}
func (w *remoteDisconnectWatcher) Disconnected(_ network.Network, conn network.Conn) {
	if conn.RemotePeer() != w.target {
		return
	}

	log.Printf("Remote peer %s disconnected", w.target)
	if w.notify != nil {
		w.notify("remote peer disconnected")
	}
}
