package server

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lachierussell/shipyard/config"
)

func testServer(cfg *config.Config) *Server {
	return &Server{
		cfg:     cfg,
		version: "1.0.0-test",
		commit:  "abc1234",
	}
}

func TestHealth_ReturnsStatus(t *testing.T) {
	srv := testServer(&config.Config{})

	app := fiber.New()
	app.Get("/health", srv.Health)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", result["status"])
	}
	if result["version"] != "1.0.0-test" {
		t.Errorf("version = %v, want 1.0.0-test", result["version"])
	}
	if result["commit"] != "abc1234" {
		t.Errorf("commit = %v, want abc1234", result["commit"])
	}
}

func TestStatus_ExistingSite(t *testing.T) {
	srv := testServer(&config.Config{
		Site: map[string]config.SiteConfig{
			"example.com": {FrontendRoot: "/var/www/example"},
		},
	})

	app := fiber.New()
	app.Get("/status/:site", srv.Status)

	req := httptest.NewRequest("GET", "/status/example.com", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["site"] != "example.com" {
		t.Errorf("site = %v, want example.com", result["site"])
	}
}

func TestStatus_ExistingSiteWithBackend(t *testing.T) {
	srv := testServer(&config.Config{
		Site: map[string]config.SiteConfig{
			"api.example.com": {
				FrontendRoot: "/var/www/api",
				Backend: &config.BackendConfig{
					JailName: "api-example-com",
				},
			},
		},
	})

	app := fiber.New()
	app.Get("/status/:site", srv.Status)

	req := httptest.NewRequest("GET", "/status/api.example.com", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["site"] != "api.example.com" {
		t.Errorf("site = %v, want api.example.com", result["site"])
	}

	backend, ok := result["backend"].(map[string]interface{})
	if !ok {
		t.Fatal("expected backend map in response")
	}
	if backend["jail"] != "api-example-com" {
		t.Errorf("backend.jail = %v, want api-example-com", backend["jail"])
	}
}

func TestStatus_SiteNotFound(t *testing.T) {
	srv := testServer(&config.Config{
		Site: map[string]config.SiteConfig{},
	})

	app := fiber.New()
	app.Get("/status/:site", srv.Status)

	req := httptest.NewRequest("GET", "/status/unknown.com", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Status = %d, want 404", resp.StatusCode)
	}
}

func TestValidDomain(t *testing.T) {
	tests := []struct {
		domain string
		valid  bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"my-app.example.com", true},
		{"a1.b2.c3", true},
		{"ab", true},
		{"a", false},          // single char (regex requires at least 2)
		{"Example.com", false}, // uppercase
		{"-example.com", false}, // leading hyphen
		{"example.com-", false}, // trailing hyphen
		{".example.com", false}, // leading dot
		{"example.com.", false}, // trailing dot
		{"exam ple.com", false}, // space
		{"example..com", true},  // double dot (valid per regex)
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := validDomain.MatchString(tt.domain)
			if got != tt.valid {
				t.Errorf("validDomain(%q) = %v, want %v", tt.domain, got, tt.valid)
			}
		})
	}
}
