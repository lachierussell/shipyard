package nginx

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/shipyard/shipyard/config"
)

// Validate runs "nginx -t" to check if the current config is valid
func Validate(cfg *config.Config) error {
	cmd := exec.Command(cfg.Nginx.BinaryPath, "-t")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// nginx -t outputs to stderr on error
		errMsg := strings.TrimSpace(string(output))
		return fmt.Errorf("nginx config invalid: %s", errMsg)
	}

	return nil
}

// ValidateAndGetError is like Validate but returns the error message separately
// (for API responses that need to differentiate between validation failures and other errors)
func ValidateAndGetError(cfg *config.Config) (bool, string) {
	cmd := exec.Command(cfg.Nginx.BinaryPath, "-t")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, strings.TrimSpace(string(output))
	}
	return true, ""
}
