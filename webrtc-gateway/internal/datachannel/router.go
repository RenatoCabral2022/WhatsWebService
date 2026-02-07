package datachannel

import (
	"encoding/json"
	"fmt"
	"log"
)

// Handler processes a specific command type.
type Handler func(sessionID, actionID string, payload json.RawMessage) error

// Router dispatches incoming data channel messages to registered handlers.
type Router struct {
	handlers map[string]Handler
}

// NewRouter creates a new message router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]Handler)}
}

// Register adds a handler for a specific message type.
func (r *Router) Register(msgType string, h Handler) {
	r.handlers[msgType] = h
}

// Dispatch parses a raw data channel message and routes it to the appropriate handler.
func (r *Router) Dispatch(raw []byte) error {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	h, ok := r.handlers[env.Type]
	if !ok {
		log.Printf("unknown message type: %s", env.Type)
		return nil
	}

	return h(env.SessionID, env.ActionID, env.Payload)
}
