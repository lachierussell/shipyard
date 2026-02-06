package server

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/shipyard/shipyard/nginx"
	"github.com/shipyard/shipyard/ssl"
)

// SiteInit initializes a new site (creates directories, jails, nginx config, etc)
func (s *Server) SiteInit(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_request",
		})
	}

	siteValues := form.Value["site"]
	nginxConfigValues := form.Value["nginx_config"]
	nginxFiles := form.File["nginx_config"]

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

	log.Info("site init started", "ssl", site.SSLEnabled, "has_backend", site.Backend != nil)

	// Get nginx config (from field or file)
	var nginxConfig string
	if len(nginxConfigValues) > 0 {
		nginxConfig = nginxConfigValues[0]
	} else if len(nginxFiles) > 0 {
		nginxFile := nginxFiles[0]
		src, err := nginxFile.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  "failed to read nginx_config",
			})
		}
		defer src.Close()

		nginxBytes := make([]byte, nginxFile.Size)
		if _, err := src.Read(nginxBytes); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  "failed to read nginx_config",
			})
		}
		nginxConfig = string(nginxBytes)
	}

	// For backend-only sites, auto-generate nginx config if none provided
	if site.IsBackendOnly() && nginxConfig == "" {
		if site.SSLEnabled {
			certPath, keyPath := ssl.CertPaths(siteName)
			nginxConfig = nginx.GenerateBackendProxyConfigHTTPS(siteName, site.Backend.ListenPort, site.Backend.ProxyPath, certPath, keyPath)
		} else {
			nginxConfig = nginx.GenerateBackendProxyConfig(siteName, site.Backend.ListenPort, site.Backend.ProxyPath)
		}
	}

	// Require nginx config for sites with a frontend
	if nginxConfig == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_nginx_config",
		})
	}

	// Render user-provided config as a template with site data
	if nginxConfig != "" {
		rendered, err := nginx.RenderUserConfig(nginxConfig, siteName, s.cfg)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_template_error",
				"detail": err.Error(),
			})
		}
		nginxConfig = rendered
	}

	response := fiber.Map{
		"status": "initialized",
		"site":   siteName,
	}

	// Create frontend directory (skip for backend-only sites)
	frontendDirCreated := false
	if site.HasFrontend() {
		if _, err := os.Stat(site.FrontendRoot); os.IsNotExist(err) {
			if err := os.MkdirAll(site.FrontendRoot, 0755); err == nil {
				frontendDirCreated = true
			}
		}
	}
	response["frontend_dir_created"] = frontendDirCreated

	// Create jail if backend exists
	jailCreated := false
	jailStarted := false
	if site.Backend != nil {
		if err := s.jailMgr.EnsureExists(siteName); err != nil {
			log.Error("jail creation failed", "error", err)
		} else {
			jailCreated = true
			if err := s.jailMgr.Start(siteName); err != nil {
				log.Error("jail start failed", "error", err)
			} else {
				jailStarted = true
			}
		}
	}
	response["jail_created"] = jailCreated
	response["jail_started"] = jailStarted

	// Create rc.d script if backend exists
	rcdCreated := false
	if site.Backend != nil {
		if err := s.serviceMgr.CreateBackendService(siteName); err != nil {
			log.Error("rc.d script creation failed", "error", err)
		} else {
			rcdCreated = true
			if err := s.serviceMgr.Enable(siteName); err != nil {
				log.Error("service enable failed", "error", err)
			}
		}
	}
	response["rcd_created"] = rcdCreated

	// If SSL is enabled, we need to:
	// 1. First deploy HTTP-only config to serve ACME challenges
	// 2. Obtain SSL certificate via Let's Encrypt
	// 3. Re-deploy with HTTPS transformation
	sslObtained := false
	if site.SSLEnabled {
		// Temporarily disable SSL to deploy HTTP config first
		site.SSLEnabled = false
		s.cfg.Site[siteName] = site

		// Deploy HTTP-only config
		reloaded, nginxErr, err := s.nginxMgr.DeploySiteConfig(siteName, nginxConfig)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_deployment_failed",
				"detail": err.Error(),
			})
		}
		if !reloaded {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_validation_failed",
				"detail": nginxErr,
			})
		}

		// Re-enable SSL
		site.SSLEnabled = true
		s.cfg.Site[siteName] = site

		// Obtain SSL certificate
		if err := s.sslMgr.ObtainCert(siteName); err != nil {
			response["ssl_error"] = err.Error()
		} else {
			sslObtained = true
		}
	}

	// Deploy final nginx config (with HTTPS if SSL was obtained)
	if !site.SSLEnabled || !sslObtained {
		// SSL not enabled or cert not obtained - keep HTTP config
		if !site.SSLEnabled {
			reloaded, nginxErr, err := s.nginxMgr.DeploySiteConfig(siteName, nginxConfig)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status": "error",
					"error":  "nginx_deployment_failed",
					"detail": err.Error(),
				})
			}
			response["nginx_reloaded"] = reloaded
			if !reloaded {
				response["nginx_error"] = nginxErr
			}
		} else {
			response["nginx_reloaded"] = true
			response["ssl_pending"] = true
		}
	} else {
		// SSL obtained - deploy HTTPS config
		reloaded, nginxErr, err := s.nginxMgr.DeploySiteConfig(siteName, nginxConfig)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_https_deployment_failed",
				"detail": err.Error(),
			})
		}
		response["nginx_reloaded"] = reloaded
		response["ssl_enabled"] = true
		if !reloaded {
			response["nginx_error"] = nginxErr
		}
	}

	log.Info("site init completed",
		"frontend_dir_created", frontendDirCreated,
		"jail_created", jailCreated,
		"jail_started", jailStarted,
		"rcd_created", rcdCreated,
	)
	return c.Status(fiber.StatusOK).JSON(response)
}
