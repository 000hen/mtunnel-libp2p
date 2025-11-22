package main

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type ConnToken struct {
	Network string
	ID      peer.ID
}
