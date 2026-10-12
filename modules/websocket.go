// Package modules is the native-module registry: one file per RN/Expo
// surface the sim covers. Each starts as a deterministic double (ringer's
// internal/testfake pattern); device-faithful behavior comes later, with the
// divergence recorded in FIDELITY.md the same commit.
package modules

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Invalidation is the exact wire message game-room.ts broadcasts and
// room-store.ts consumes: {"type":"room-invalidated","stateVersion":N}.
type Invalidation struct {
	Type         string `json:"type"`
	StateVersion int    `json:"stateVersion"`
}

func encodeInvalidation(version int) string {
	msg, _ := json.Marshal(Invalidation{Type: "room-invalidated", StateVersion: version})
	return string(msg)
}

// Hub is a deterministic in-memory double of the room socket surface:
// subscribe a room code, broadcast invalidations, answer pings.
// Real behavior (25s ping interval, 80ms client debounce, reconnect backoff)
// lives in the client; see the /tmp/hatch-spike transcript.
type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan string
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[string][]chan string)}
}

// Subscribe registers a buffered listener for a room code. The caller owns
// the channel; Unsubscribe is unnecessary for scenario-scoped hubs.
func (h *Hub) Subscribe(code string) <-chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan string, 16)
	h.subs[code] = append(h.subs[code], ch)
	return ch
}

// BroadcastInvalidation delivers the real wire-format message to every
// subscriber of code. It never blocks: a full listener buffer drops the
// message, mirroring a missed socket frame the client's refresh recovers.
func (h *Hub) BroadcastInvalidation(code string, stateVersion int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	msg := encodeInvalidation(stateVersion)
	for _, ch := range h.subs[code] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// Ping answers a client ping the way the worker does.
func Ping() string {
	return "pong"
}

// DecodeInvalidation parses a hub message back into its fields, failing on
// anything that is not the real wire format.
func DecodeInvalidation(raw string) (Invalidation, error) {
	var inv Invalidation
	if err := json.Unmarshal([]byte(raw), &inv); err != nil {
		return Invalidation{}, fmt.Errorf("modules: invalid invalidation %q: %w", raw, err)
	}
	if inv.Type != "room-invalidated" {
		return Invalidation{}, fmt.Errorf("modules: unexpected message type %q", inv.Type)
	}
	return inv, nil
}
