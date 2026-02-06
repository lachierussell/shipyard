package server

import (
	"bytes"

	"github.com/gofiber/fiber/v2"
	"github.com/shipyard/shipyard/nginx"
)

// NginxExample returns the example nginx override template.
// If a ?site= query param is provided, returns the rendered default config for that site.
func (s *Server) NginxExample(c *fiber.Ctx) error {
	siteName := c.Query("site")

	// If a site is specified, return its rendered default config
	if siteName != "" {
		site, ok := s.cfg.Site[siteName]
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "error",
				"error":  "site_not_found",
			})
		}

		// Generate the default config that would be used for this site
		var buf bytes.Buffer
		if err := nginxDefaultTmpl.Execute(&buf, nginxDefaultData{
			ServerName:   siteName,
			FrontendRoot: site.FrontendRoot,
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "template_error",
				"detail": err.Error(),
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"example":  nginx.GetOverrideExample(),
			"default":  buf.String(),
			"site":     siteName,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"example": nginx.GetOverrideExample(),
	})
}
