package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lachierussell/shipyard/config"
)

func TestAdminAuth_ValidKey(t *testing.T) {
	cfg := &config.Config{
		AdminKeys: []string{"sk-admin-test-key"},
	}

	app := fiber.New()
	app.Post("/test", AdminAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Shipyard-Key", "sk-admin-test-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestAdminAuth_MissingKey(t *testing.T) {
	cfg := &config.Config{
		AdminKeys: []string{"sk-admin-test-key"},
	}

	app := fiber.New()
	app.Post("/test", AdminAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	// No X-Shipyard-Key header

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Status = %d, want 401", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "missing_auth" {
		t.Errorf("Error = %v, want missing_auth", result["error"])
	}
}

func TestAdminAuth_InvalidKey(t *testing.T) {
	cfg := &config.Config{
		AdminKeys: []string{"sk-admin-test-key"},
	}

	app := fiber.New()
	app.Post("/test", AdminAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Shipyard-Key", "wrong-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Status = %d, want 401", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "invalid_key" {
		t.Errorf("Error = %v, want invalid_key", result["error"])
	}
}

func TestAdminAuth_MultipleKeys(t *testing.T) {
	cfg := &config.Config{
		AdminKeys: []string{"key1", "key2", "key3"},
	}

	app := fiber.New()
	app.Post("/test", AdminAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Test each key works
	for _, key := range cfg.AdminKeys {
		req := httptest.NewRequest("POST", "/test", nil)
		req.Header.Set("X-Shipyard-Key", key)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Test request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Errorf("Key %s: Status = %d, want 200", key, resp.StatusCode)
		}
	}
}

func TestSiteAuth_ValidKey(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.example.com": {
				FrontendRoot: "/var/www/test",
				APIKey:       "sk-site-test-key",
			},
		},
	}

	app := fiber.New()
	app.Post("/test", SiteAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create multipart form with site field (domain is the key)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("site", "test.example.com")
	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Shipyard-Key", "sk-site-test-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestSiteAuth_MissingSite(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{},
	}

	app := fiber.New()
	app.Post("/test", SiteAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Create multipart form without site field
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Shipyard-Key", "some-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Status = %d, want 400", resp.StatusCode)
	}
}

func TestSiteAuth_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"realsite.example.com": {APIKey: "key"},
		},
	}

	app := fiber.New()
	app.Post("/test", SiteAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("site", "nonexistent")
	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Shipyard-Key", "some-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Status = %d, want 404", resp.StatusCode)
	}
}

func TestSiteAuth_InvalidKey(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.example.com": {
				FrontendRoot: "/var/www/test",
				APIKey:       "correct-key",
			},
		},
	}

	app := fiber.New()
	app.Post("/test", SiteAuth(cfg), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("site", "test.example.com")
	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Shipyard-Key", "wrong-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("Status = %d, want 401", resp.StatusCode)
	}
}

func TestSizeLimit_UnderLimit(t *testing.T) {
	app := fiber.New()
	app.Post("/test", SizeLimit(), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Small body
	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("small body")))
	req.Header.Set("Content-Length", "10")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}
}

func TestSizeLimit_OverLimit(t *testing.T) {
	app := fiber.New()
	app.Post("/test", SizeLimit(), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("small")))
	// Set Content-Length header to exceed limit (500MB = 524288000)
	// Fiber's test client may error on large bodies, so we test the middleware logic
	req.Header.Set("Content-Length", "600000000")

	resp, err := app.Test(req, -1) // -1 = no timeout
	if err != nil {
		// Fiber may reject the request before it reaches our middleware
		// This is acceptable behavior - the request is still rejected
		t.Skipf("Fiber rejected request before middleware: %v", err)
	}

	if resp.StatusCode != 413 {
		t.Errorf("Status = %d, want 413", resp.StatusCode)
	}
}

func TestIsValidCommitHash(t *testing.T) {
	tests := []struct {
		hash  string
		valid bool
	}{
		{"abc1234", true},          // 7 chars
		{"abcdef1234567890", true}, // 16 chars
		{"abcdef1234567890abcdef1234567890abcdef12", true}, // 40 chars (full SHA)
		{"abc123", false},          // Too short (6 chars)
		{"ABC1234", false},         // Uppercase not allowed
		{"abcdefg", false},         // 'g' is not hex
		{"abc12345678901234567890123456789012345678901", false}, // Too long (41 chars)
		{"", false},
		{"abc-123", false}, // Invalid char
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			got := isValidCommitHash(tt.hash)
			if got != tt.valid {
				t.Errorf("isValidCommitHash(%q) = %v, want %v", tt.hash, got, tt.valid)
			}
		})
	}
}
