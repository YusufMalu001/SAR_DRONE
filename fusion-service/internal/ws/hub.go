// Package ws implements the /ws/live broadcast endpoint: every fused
// detection the fusion pipeline persists is pushed to all connected
// clients in real time.
package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Prototype: the dashboard is served from a different origin during
	// local dev (Vite on :5173 vs this service on :8002), so allow all.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub fans out broadcast payloads to every connected /ws/live client.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// Broadcast sends payload to every connected client. A slow client that
// hasn't drained its buffer has this message dropped for it rather than
// blocking the fusion pipeline.
func (h *Hub) Broadcast(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
		}
	}
}

// HandleLive upgrades the request to a WebSocket and streams broadcast
// payloads to it until the client disconnects.
func (h *Hub) HandleLive(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ch := make(chan []byte, 32)
	h.register(ch)
	defer h.unregister(ch)

	// This endpoint is send-only, but we still need to notice when the
	// client goes away (close frame, dropped connection, etc).
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				h.unregister(ch)
				return
			}
		}
	}()

	for payload := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (h *Hub) register(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

// unregister removes ch from the client set and closes it, if it hasn't
// already been removed. Deleting-then-closing under the same lock that
// Broadcast holds while sending guarantees Broadcast never sends on a
// closed channel.
func (h *Hub) unregister(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}
