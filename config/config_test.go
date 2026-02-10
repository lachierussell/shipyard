package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
admin_keys = ["sk-test-admin-key"]

[server]
listen_addr = "0.0.0.0:8080"

[nginx]
binary_path = "/usr/sbin/nginx"
main_conf_path = "/etc/nginx/nginx.conf"
sites_available = "/etc/nginx/sites-available"
sites_enabled = "/etc/nginx/sites-enabled"

[jail]
base_dir = "/var/jails"
jail_conf_path = "/etc/jail.conf"

[site."test.example.com"]
frontend_root = "/var/www/test"
api_key = "sk-site-test-key"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.ListenAddr != "0.0.0.0:8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.Server.ListenAddr, "0.0.0.0:8080")
	}

	if len(cfg.AdminKeys) != 1 || cfg.AdminKeys[0] != "sk-test-admin-key" {
		t.Errorf("AdminKeys = %v, want [sk-test-admin-key]", cfg.AdminKeys)
	}

	if len(cfg.Site) != 1 {
		t.Errorf("Site count = %d, want 1", len(cfg.Site))
	}

	// Domain is now the key
	_, ok := cfg.Site["test.example.com"]
	if !ok {
		t.Fatal("Site 'test.example.com' not found")
	}
}

func TestLoad_JailBinaryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
admin_keys = ["sk-test-admin-key"]

[server]
listen_addr = "0.0.0.0:8080"

[nginx]
binary_path = "/usr/sbin/nginx"
main_conf_path = "/etc/nginx/nginx.conf"
sites_available = "/etc/nginx/sites-available"
sites_enabled = "/etc/nginx/sites-enabled"

[jail]
binary_path = "/usr/local/bin/pot"
base_dir = "/var/jails"
jail_conf_path = "/etc/jail.conf"

[site."test.example.com"]
frontend_root = "/var/www/test"
api_key = "sk-site-test-key"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Jail.BinaryPath != "/usr/local/bin/pot" {
		t.Errorf("Jail.BinaryPath = %q, want %q", cfg.Jail.BinaryPath, "/usr/local/bin/pot")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("Load() should fail for nonexistent file")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.toml")

	content := `this is not valid toml {{{`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() should fail for invalid TOML")
	}
}

func TestValidate_MissingListenAddr(t *testing.T) {
	cfg := &Config{
		AdminKeys: []string{"key"},
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site:      map[string]SiteConfig{"test.example.com": {FrontendRoot: "/f", APIKey: "k"}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail with missing listen_addr")
	}
}

func TestValidate_MissingAdminKeys(t *testing.T) {
	cfg := &Config{
		Server:    ServerConfig{ListenAddr: ":8080"},
		AdminKeys: []string{}, // Empty
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site:      map[string]SiteConfig{"test.example.com": {FrontendRoot: "/f", APIKey: "k"}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail with empty admin_keys")
	}
}

func TestValidate_MissingSite(t *testing.T) {
	cfg := &Config{
		Server:    ServerConfig{ListenAddr: ":8080"},
		AdminKeys: []string{"key"},
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site:      map[string]SiteConfig{}, // Empty
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail with empty Site map")
	}
}

func TestValidate_SiteMissingFrontendRoot_NoBackend(t *testing.T) {
	cfg := &Config{
		Server:    ServerConfig{ListenAddr: ":8080"},
		AdminKeys: []string{"key"},
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site:      map[string]SiteConfig{"test.example.com": {APIKey: "k"}}, // Missing FrontendRoot, no backend
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail with missing frontend_root and no backend")
	}
}

func TestValidate_BackendOnly_NoFrontendRoot(t *testing.T) {
	cfg := &Config{
		Server:    ServerConfig{ListenAddr: ":8080"},
		AdminKeys: []string{"key"},
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site: map[string]SiteConfig{
			"api.example.com": {
				APIKey: "k",
				Backend: &BackendConfig{
					JailName:   "api-example-com",
					JailIP:     "127.0.1.1",
					ListenPort: 8080,
					BinaryName: "myapi",
				},
			},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should accept backend-only site without frontend_root, got error: %v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Server:    ServerConfig{ListenAddr: ":8080"},
		AdminKeys: []string{"key"},
		Nginx:     NginxConfig{BinaryPath: "/x", MainConfPath: "/x", SitesAvailable: "/x", SitesEnabled: "/x"},
		Jail:      JailConfig{BaseDir: "/x", JailConfPath: "/x"},
		Site:      map[string]SiteConfig{"test.example.com": {FrontendRoot: "/f", APIKey: "k"}},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestGetSite_Found(t *testing.T) {
	cfg := &Config{
		Site: map[string]SiteConfig{
			"example.com": {FrontendRoot: "/var/www", APIKey: "key"},
		},
	}

	site, err := cfg.GetSite("example.com")
	if err != nil {
		t.Fatalf("GetSite() error = %v", err)
	}

	if site.FrontendRoot != "/var/www" {
		t.Errorf("GetSite().FrontendRoot = %q, want %q", site.FrontendRoot, "/var/www")
	}
}

func TestGetSite_NotFound(t *testing.T) {
	cfg := &Config{
		Site: map[string]SiteConfig{},
	}

	_, err := cfg.GetSite("nonexistent")
	if err == nil {
		t.Error("GetSite() should fail for nonexistent site")
	}
}

func TestGetSiteWithBackend_Found(t *testing.T) {
	cfg := &Config{
		Site: map[string]SiteConfig{
			"example.com": {
				FrontendRoot: "/var/www",
				APIKey:       "key",
				Backend: &BackendConfig{
					JailName:   "myjail",
					JailIP:     "127.0.1.1",
					ListenPort: 8080,
				},
			},
		},
	}

	site, err := cfg.GetSiteWithBackend("example.com")
	if err != nil {
		t.Fatalf("GetSiteWithBackend() error = %v", err)
	}

	if site.Backend.JailName != "myjail" {
		t.Errorf("GetSiteWithBackend().Backend.JailName = %q, want %q", site.Backend.JailName, "myjail")
	}
}

func TestGetSiteWithBackend_NoBackend(t *testing.T) {
	cfg := &Config{
		Site: map[string]SiteConfig{
			"example.com": {FrontendRoot: "/var/www", APIKey: "key"},
		},
	}

	_, err := cfg.GetSiteWithBackend("example.com")
	if err == nil {
		t.Error("GetSiteWithBackend() should fail for site without backend")
	}
}

func TestSiteConfig_HasFrontend(t *testing.T) {
	tests := []struct {
		name string
		site SiteConfig
		want bool
	}{
		{"with frontend", SiteConfig{FrontendRoot: "/var/www"}, true},
		{"no frontend", SiteConfig{FrontendRoot: ""}, false},
		{"backend only", SiteConfig{Backend: &BackendConfig{JailName: "x", JailIP: "127.0.1.1", ListenPort: 8080, BinaryName: "x"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.site.HasFrontend(); got != tt.want {
				t.Errorf("HasFrontend() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSiteConfig_IsBackendOnly(t *testing.T) {
	tests := []struct {
		name string
		site SiteConfig
		want bool
	}{
		{"frontend only", SiteConfig{FrontendRoot: "/var/www"}, false},
		{"frontend + backend", SiteConfig{FrontendRoot: "/var/www", Backend: &BackendConfig{}}, false},
		{"backend only", SiteConfig{Backend: &BackendConfig{}}, true},
		{"no frontend no backend", SiteConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.site.IsBackendOnly(); got != tt.want {
				t.Errorf("IsBackendOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}
