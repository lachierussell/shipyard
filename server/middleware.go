package server

import (
	"crypto/subtle"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shipyard/shipyard/config"
)

// AdminAuth checks X-Shipyard-Key against admin_keys
func AdminAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Get("X-Shipyard-Key")
		if key == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status": "error",
				"error":  "missing_auth",
				"detail": "X-Shipyard-Key header required",
			})
		}

		// Check if key is in admin_keys (constant-time comparison)
		found := false
		for _, adminKey := range cfg.AdminKeys {
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
}

// SiteAuth checks X-Shipyard-Key against the targeted site's api_key OR admin_keys
// The site is identified from the "site" form field
// Admin keys can perform any site operation
func SiteAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the body to get the site field
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  "invalid_request",
				"detail": "failed to parse form",
			})
		}

		siteName := form.Value["site"]
		if len(siteName) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  "missing_site",
			})
		}

		site, ok := cfg.Site[siteName[0]]
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "error",
				"error":  "site_not_found",
			})
		}

		key := c.Get("X-Shipyard-Key")
		if key == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status": "error",
				"error":  "missing_auth",
			})
		}

		// Check if key matches site API key
		if subtle.ConstantTimeCompare([]byte(key), []byte(site.APIKey)) == 1 {
			return c.Next()
		}

		// Also allow admin keys to perform site operations
		for _, adminKey := range cfg.AdminKeys {
			if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) == 1 {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_key",
		})
	}
}

// RequestLogger logs incoming requests with method, path, status, and latency.
// It assigns a unique request ID accessible via X-Request-Id header and c.Locals("request_id").
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		requestID := uuid.NewString()
		c.Set("X-Request-Id", requestID)
		c.Locals("request_id", requestID)

		// Store a request-scoped logger in locals
		reqLogger := slog.Default().With("request_id", requestID)
		c.Locals("logger", reqLogger)

		err := c.Next()

		reqLogger.Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_size", len(c.Body()),
			"response_size", len(c.Response().Body()),
		)

		return err
	}
}

// CORS handles cross-origin requests for the web admin UI
func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, X-Shipyard-Key")

		// Handle preflight
		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// SizeLimit rejects requests larger than MaxRequestSize
func SizeLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len := c.Request().Header.ContentLength(); len > MaxRequestSize {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"status": "error",
				"error":  "request_too_large",
				"detail": "max 500MB",
			})
		}
		return c.Next()
	}
}
