package server

import (
	"github.com/gofiber/fiber/v2"
)

// DeployBackend handles POST /deploy/backend
func (s *Server) DeployBackend(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_request",
		})
	}

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

	// Validate site exists and has backend config
	site, ok := s.cfg.Site[siteName]
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "error",
			"error":  "site_not_found",
		})
	}

	if site.Backend == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "site_has_no_backend",
		})
	}

	// Validate commit hash
	if !isValidCommitHash(commitHash) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_commit_hash",
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

	// Open artifact
	src, err := artifactFile.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "failed to read artifact",
		})
	}
	defer src.Close()

	// Determine binary name (default to site name if not specified)
	binaryName := site.Backend.BinaryName
	if binaryNameValues := form.Value["binary_name"]; len(binaryNameValues) > 0 {
		binaryName = binaryNameValues[0]
	}

	log := reqLog(c).With("site", siteName, "commit", commitHash, "jail", site.Backend.JailName)
	log.Info("backend deploy started")

	// Deploy
	if err := s.backendDeployer.Deploy(siteName, commitHash, src, binaryName); err != nil {
		log.Error("backend deploy failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "deployment_failed",
			"detail": err.Error(),
		})
	}

	log.Info("backend deploy succeeded")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "deployed",
		"site":     siteName,
		"commit":   commitHash,
		"jail":     site.Backend.JailName,
		"healthy":  true,
	})
}
