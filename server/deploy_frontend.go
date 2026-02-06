package server

import (
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"text/template"

	"github.com/gofiber/fiber/v2"
	"github.com/shipyard/shipyard/nginx"
)

// commitHashRegex validates git commit hashes (7-40 hex chars)
var commitHashRegex = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

type nginxDefaultData struct {
	ServerName   string
	FrontendRoot string
}

//go:embed nginx_default.conf.tmpl
var nginxDefaultTmplStr string

var nginxDefaultTmpl = template.Must(template.New("nginx_default").Delims("<%", "%>").Parse(nginxDefaultTmplStr))

// DeployFrontend handles POST /deploy/frontend
func (s *Server) DeployFrontend(c *fiber.Ctx) error {
	// Parse form data
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_request",
			"detail": "failed to parse multipart form",
		})
	}

	// Get form fields
	siteValues := form.Value["site"]
	commitValues := form.Value["commit"]
	if len(siteValues) == 0 || len(commitValues) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_fields",
		})
	}

	siteName := siteValues[0]
	commitHash := commitValues[0]

	// Check if we should update the 'latest' symlink (defaults to false for branch previews)
	updateLatest := false
	if updateLatestValues := form.Value["update_latest"]; len(updateLatestValues) > 0 {
		updateLatest = updateLatestValues[0] == "true" || updateLatestValues[0] == "1"
	}

	// Validate site exists
	site, ok := s.cfg.Site[siteName]
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "error",
			"error":  "site_not_found",
		})
	}

	// Reject frontend deploys for backend-only sites
	if site.IsBackendOnly() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "backend_only_site",
			"detail": "this site has no frontend; use /deploy/backend instead",
		})
	}

	// Validate commit hash format (7-40 char hex)
	if !isValidCommitHash(commitHash) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_commit_hash",
			"detail": "must be 7-40 char hex string",
		})
	}

	// Get artifact file
	files := form.File["artifact"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_artifact",
		})
	}

	artifactFile := files[0]

	// Get nginx config (optional - will use default if not provided)
	var nginxConfig string
	nginxConfigValues := form.Value["nginx_config"]
	if len(nginxConfigValues) > 0 && nginxConfigValues[0] != "" {
		nginxConfig = nginxConfigValues[0]
	} else {
		// Try as file
		nginxFiles := form.File["nginx_config"]
		if len(nginxFiles) > 0 {
			nginxFile := nginxFiles[0]
			src, err := nginxFile.Open()
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"status": "error",
					"error":  "failed to read nginx_config file",
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
	}

	// Use default nginx config if none provided
	if nginxConfig == "" {
		var buf bytes.Buffer
		if err := nginxDefaultTmpl.Execute(&buf, nginxDefaultData{
			ServerName:   siteName,
			FrontendRoot: site.FrontendRoot,
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_config_generation_failed",
				"detail": err.Error(),
			})
		}
		nginxConfig = buf.String()
	} else {
		// Render user-provided config as a template with site data
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

	// Open the artifact file
	src, err := artifactFile.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "failed to read artifact",
		})
	}
	defer src.Close()

	log := reqLog(c).With("site", siteName, "commit", commitHash)
	log.Info("frontend deploy started")

	// Check that frontend root exists (site must be initialized)
	reloaded, nginxErr, err := s.frontendDeployer.Deploy(siteName, commitHash, src, nginxConfig, updateLatest)

	if err != nil {
		log.Error("frontend deploy failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "deployment_failed",
			"detail": err.Error(),
		})
	}

	if !reloaded {
		log.Warn("frontend deploy partial: nginx validation failed", "nginx_error", nginxErr)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"status":              "partially_deployed",
			"error":               "nginx_validation_failed",
			"detail":              nginxErr,
			"commit_deployed":     true,
			"nginx_reloaded":      false,
			"latest_updated":      updateLatest,
			"site":                siteName,
			"commit":              commitHash,
		})
	}

	log.Info("frontend deploy succeeded", "update_latest", updateLatest)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":          "deployed",
		"site":            siteName,
		"commit":          commitHash,
		"path":            fmt.Sprintf("%s/%s", site.FrontendRoot, commitHash),
		"nginx_reloaded":  true,
		"latest_updated":  updateLatest,
	})
}

// isValidCommitHash checks if a string is a valid git commit hash (7-40 hex chars) or "latest"
func isValidCommitHash(hash string) bool {
	return hash == "latest" || commitHashRegex.MatchString(hash)
}
