package server

import (
	"github.com/gofiber/websocket/v2"
)

// LogClient is a single WebSocket subscriber.
type LogClient struct {
	conn *websocket.Conn
	send chan []byte
}

// LogHub manages WebSocket log subscribers using the hub pattern.
// It implements logger.Broadcaster.
type LogHub struct {
	clients    map[*LogClient]struct{}
	register   chan *LogClient
	unregister chan *LogClient
	broadcast  chan []byte
	stop       chan struct{}
}

// NewLogHub creates a new LogHub.
func NewLogHub() *LogHub {
	return &LogHub{
		clients:    make(map[*LogClient]struct{}),
		register:   make(chan *LogClient),
		unregister: make(chan *LogClient),
		broadcast:  make(chan []byte, 256),
		stop:       make(chan struct{}),
	}
}

// Run processes register/unregister/broadcast events. Call in a goroutine.
func (h *LogHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case msg := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Slow client — drop and disconnect.
					delete(h.clients, client)
					close(client.send)
				}
			}
		case <-h.stop:
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return
		}
	}
}

// Broadcast sends a message to all connected clients (implements logger.Broadcaster).
func (h *LogHub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// Hub broadcast channel full — drop message.
	}
}

// Stop shuts down the hub's Run loop.
func (h *LogHub) Stop() {
	close(h.stop)
}
