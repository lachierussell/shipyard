package server

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// WSLogsUpgrade is middleware that validates the admin key from a query param
// and marks the request for WebSocket upgrade.
func (s *Server) WSLogsUpgrade(c *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}

	key := c.Query("key")
	if key == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_auth",
			"detail": "key query parameter required",
		})
	}

	found := false
	for _, adminKey := range s.cfg.AdminKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) == 1 {
			found = true
			break
		}
	}
	if !found {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_key",
		})
	}

	return c.Next()
}

// WSLogs handles a WebSocket connection for streaming log entries.
func (s *Server) WSLogs(c *websocket.Conn) {
	client := &LogClient{
		conn: c,
		send: make(chan []byte, 64),
	}

	s.logHub.register <- client

	// writePump: send messages from channel to WebSocket.
	go func() {
		defer c.Close()
		for msg := range client.send {
			if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()

	// readPump: detect client disconnect.
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}

	s.logHub.unregister <- client
}
