package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SiteInfo contains site configuration and health status
type SiteInfo struct {
	Domain       string `json:"domain"`
	FrontendRoot string `json:"frontend_root"`
	HasBackend   bool   `json:"has_backend"`
	BackendOnly  bool   `json:"backend_only"`
	SSLEnabled   bool   `json:"ssl_enabled"`
	Health       string `json:"health"` // "healthy", "unhealthy", "unknown"
}

// ListSites returns all configured sites with their health status (admin only)
func (s *Server) ListSites(c *fiber.Ctx) error {
	sites := make([]SiteInfo, 0, len(s.cfg.Site))

	for domain, site := range s.cfg.Site {
		info := SiteInfo{
			Domain:       domain,
			FrontendRoot: site.FrontendRoot,
			HasBackend:   site.Backend != nil,
			BackendOnly:  site.IsBackendOnly(),
			SSLEnabled:   site.SSLEnabled,
			Health:       checkSiteHealth(domain, site.SSLEnabled),
		}
		sites = append(sites, info)
	}

	return c.JSON(fiber.Map{
		"sites": sites,
	})
}

// checkSiteHealth performs a quick health check on the site
func checkSiteHealth(domain string, sslEnabled bool) string {
	scheme := "http"
	if sslEnabled {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s/health", scheme, domain)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}
	return "unhealthy"
}
