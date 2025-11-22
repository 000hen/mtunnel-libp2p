package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	TOKEN      = "TOKEN"
	DISCONNECT = "DISCONNECT"
	CONNECTED  = "CONNECTED"
	LIST       = "LIST"
	ERROR      = "ERROR"
)

type InputAction struct {
	Action    string  `json:"action"`
	SessionId peer.ID `json:"session_id"`
}

type OutputAction struct {
	Action    string    `json:"action"`
	SessionId peer.ID   `json:"session_id,omitempty"`
	Sessions  []peer.ID `json:"sessions,omitempty"`
	Token     string    `json:"token,omitempty"`
	Addr      string    `json:"addr,omitempty"`
	Port      int       `json:"port,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func handleIOAction(sessionManager *SessionManager) {
	decoder := json.NewDecoder(os.Stdin)

	for {
		var rawInput json.RawMessage
		err := decoder.Decode(&rawInput)
		if err != nil {
			if err.Error() == "EOF" {
				log.Println("Input stream closed, stopping IO action handler")
				return
			}

			log.Printf("Error decoding input action: %v", err)
			continue
		}

		var input InputAction
		err = json.Unmarshal(rawInput, &input)
		if err != nil {
			log.Printf("Error unmarshaling input action: %v", err)
			continue
		}

		switch input.Action {
		case LIST:
			sessions := sessionManager.ListSessions()
			output := OutputAction{
				Action:   LIST,
				Sessions: sessions,
			}

			sendOutputAction(output)
		case DISCONNECT:
			sessionManager.RemoveSession(input.SessionId)
			log.Printf("Session %s disconnected successfully", input.SessionId)

			output := OutputAction{
				Action:    DISCONNECT,
				SessionId: input.SessionId,
			}

			sendOutputAction(output)
		default:
			log.Printf("Unknown action received: %s", input.Action)
		}
	}
}

func sendOutputAction(output OutputAction) {
	data, err := json.Marshal(output)
	if err != nil {
		log.Printf("Error marshaling output action: %v", err)
		return
	}

	data = append(data, '\n')

	_, err = os.Stdout.Write(data)
	if err != nil {
		log.Printf("Error writing output action to stdout: %v", err)
		return
	}
}
