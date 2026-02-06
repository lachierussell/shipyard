package server

import (
	"bytes"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DeploySelf handles shipyard's own update
func (s *Server) DeploySelf(c *fiber.Ctx) error {
	log := reqLog(c)

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "request body is empty",
		})
	}

	log.Info("self-update started", "binary_size", len(body))

	// Perform the update
	if err := s.updater.Update(bytes.NewReader(body)); err != nil {
		log.Error("self-update failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	log.Info("self-update succeeded, scheduling restart")

	// Schedule graceful shutdown after response is sent
	go func() {
		// Small delay to ensure response is sent
		time.Sleep(100 * time.Millisecond)
		s.TriggerShutdown()
	}()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "restarting",
		"message": "Update successful, restarting...",
	})
}
