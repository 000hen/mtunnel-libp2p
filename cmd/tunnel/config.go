package main

import (
	"fmt"
	"time"
)

const (
	protocolID                = "/mtunnel/1.0.0"
	networkStabilizationDelay = 20 * time.Second
	defaultLocalDialTimeout   = 10 * time.Second
)

var supportedNetworks = map[string]struct{}{
	"tcp": {},
	"udp": {},
}

func validateNetworkType(network string) error {
	if _, ok := supportedNetworks[network]; ok {
		return nil
	}

	return fmt.Errorf("unsupported network type %q", network)
}
