package main

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Session struct {
	ID        peer.ID
	Conn      network.Conn
	CreatedAt time.Time
}

type SessionManager struct {
	mu       sync.Mutex
	Sessions map[peer.ID]*Session
	Network  network.Network
}

func NewSessionManager(network network.Network) *SessionManager {
	return &SessionManager{
		Sessions: make(map[peer.ID]*Session),
		Network:  network,
	}
}

func (sm *SessionManager) AddSession(id peer.ID, conn network.Conn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Sessions[id] = &Session{
		ID:        id,
		Conn:      conn,
		CreatedAt: time.Now(),
	}

	sendOutputAction(OutputAction{
		Action:    CONNECTED,
		SessionId: id,
		Addr:      conn.RemoteMultiaddr().String(),
	})
}

func (sm *SessionManager) RemoveSession(id peer.ID, isDisconnect bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, conn := range sm.Network.ConnsToPeer(id) {
		for _, s := range conn.GetStreams() {
			s.Reset()
		}
	}

	if isDisconnect {
		sm.Network.ClosePeer(id)
	}

	delete(sm.Sessions, id)
	sendOutputAction(OutputAction{
		Action:    DISCONNECT,
		SessionId: id,
	})
}

func (sm *SessionManager) ListSessions() []peer.ID {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var sessionIDs []peer.ID
	for id := range sm.Sessions {
		sessionIDs = append(sessionIDs, id)
	}

	return sessionIDs
}

func (sm *SessionManager) ForceCloseAllSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id := range sm.Sessions {
		for _, conn := range sm.Network.ConnsToPeer(id) {
			for _, s := range conn.GetStreams() {
				s.Reset()
			}
		}
		sm.Network.ClosePeer(id)
	}

	sm.Sessions = make(map[peer.ID]*Session)
}

func (s *Session) Age() time.Duration {
	return time.Since(s.CreatedAt)
}
