package server

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

// SiteDestroy tears down a site completely
func (s *Server) SiteDestroy(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_request",
		})
	}

	siteValues := form.Value["site"]
	if len(siteValues) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_site",
		})
	}

	siteName := siteValues[0]
	log := reqLog(c).With("site", siteName)

	site, ok := s.cfg.Site[siteName]
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "error",
			"error":  "site_not_found",
		})
	}

	log.Info("site destroy started")

	// Stop and disable service
	if site.Backend != nil {
		s.serviceMgr.Stop(siteName)
		s.serviceMgr.Disable(siteName)
		s.serviceMgr.RemoveBackendService(siteName)

		// Destroy jail
		s.jailMgr.Destroy(siteName)
	}

	// Remove nginx config
	if err := s.nginxMgr.RemoveSiteConfig(siteName); err != nil {
		log.Warn("failed to remove nginx config", "error", err)
	}

	// Remove frontend directory (skip for backend-only sites)
	if site.HasFrontend() {
		os.RemoveAll(site.FrontendRoot)
	}

	// Remove site from config
	configRemoved := false
	if err := s.cfg.RemoveSite(siteName); err != nil {
		log.Error("failed to remove site from config", "error", err)
	} else {
		configRemoved = true
	}

	log.Info("site destroyed", "config_removed", configRemoved)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":         "destroyed",
		"site":           siteName,
		"config_removed": configRemoved,
	})
}
