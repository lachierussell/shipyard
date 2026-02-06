package server

import "github.com/gofiber/fiber/v2"

// Health returns the health status of shipyard and its services
func (s *Server) Health(c *fiber.Ctx) error {
	// Basic health check - in production with a monitor, this would
	// include service status from the health monitor
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "healthy",
		"version":  s.version,
		"commit":   s.commit,
		"services": make(map[string]interface{}),
	})
}

// Status returns the status of a specific site
func (s *Server) Status(c *fiber.Ctx) error {
	siteName := c.Params("site")

	site, ok := s.cfg.Site[siteName]
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "error",
			"error":  "site_not_found",
		})
	}

	response := fiber.Map{
		"site": siteName,
	}

	// Basic backend status
	if site.Backend != nil {
		response["backend"] = fiber.Map{
			"jail":   site.Backend.JailName,
			"status": "unknown", // Would be updated by health monitor in production
		}
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
