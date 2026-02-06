package server

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// SiteLogs returns the last N lines of a site's application log
func (s *Server) SiteLogs(c *fiber.Ctx) error {
	log := reqLog(c)

	siteName := c.Query("site")
	if siteName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing site parameter",
		})
	}

	if _, ok := s.cfg.Site[siteName]; !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status": "error",
			"error":  "site not found",
		})
	}

	maxLines := 200
	if q := c.Query("lines"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			if n > 5000 {
				n = 5000
			}
			maxLines = n
		}
	}

	potPath, err := s.jailMgr.GetPotPath(siteName)
	if err != nil {
		log.Warn("get pot path for logs", "site", siteName, "error", err)
		return c.JSON(fiber.Map{
			"status": "ok",
			"site":   siteName,
			"lines":  []string{},
		})
	}

	logFile := filepath.Join(potPath, "m", "var", "log", "app.log")
	lines, err := tailFile(logFile, maxLines)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(fiber.Map{
				"status": "ok",
				"site":   siteName,
				"lines":  []string{},
			})
		}
		log.Warn("read site log", "site", siteName, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "failed to read log file",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"site":   siteName,
		"lines":  lines,
	})
}

// tailFile reads the last n lines from a file
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	// Increase buffer size for long log lines
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}
