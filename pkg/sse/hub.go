// Package sse provides a small in-memory Server-Sent Events broadcaster.
// It is intentionally dependency-free so it can be reused for any real-time
// admin feature (new orders, stock alerts, etc.).
package sse

import "sync"

// Message is a single SSE payload. Event maps to the SSE "event:" field and
// Data to the "data:" field (already serialized, typically JSON).
type Message struct {
	Event string
	Data  []byte
}

// Hub fans out messages to every connected subscriber. It is safe for
// concurrent use.
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]chan Message
	nextID  int64
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[int64]chan Message)}
}

// Subscribe registers a new client and returns its id plus a receive-only
// channel. Callers must call Unsubscribe(id) when done.
func (h *Hub) Subscribe() (int64, <-chan Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	// Buffered so a momentarily slow client does not block Broadcast.
	ch := make(chan Message, 16)
	h.clients[id] = ch
	return id, ch
}

// Unsubscribe removes a client and closes its channel. Safe to call twice.
func (h *Hub) Unsubscribe(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.clients[id]; ok {
		delete(h.clients, id)
		close(ch)
	}
}

// Broadcast delivers msg to every subscriber. Clients whose buffer is full are
// skipped (the message is dropped for them) to keep the broadcaster non-blocking.
func (h *Hub) Broadcast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// ClientCount returns the number of currently connected subscribers.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
