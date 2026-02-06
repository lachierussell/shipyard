package nginx

import (
	"strings"
	"testing"

	"github.com/lachierussell/shipyard/config"
)

func TestNormalizeDomainName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example_com"},
		{"sub.example.com", "sub_example_com"},
		{"a.b.c.d.example.com", "a_b_c_d_example_com"},
		{"localhost", "localhost"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeDomainName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeDomainName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateOverrideConf_HasManagedHeader(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.example.com": {},
		},
	}

	result := GenerateOverrideConf(cfg)

	if !strings.Contains(result, "MANAGED BY SHIPYARD") {
		t.Error("GenerateOverrideConf() should include managed header")
	}
}

func TestGenerateOverrideConf_HasVersionMap(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.example.com": {},
		},
	}

	result := GenerateOverrideConf(cfg)

	if !strings.Contains(result, "map $arg_override $frontend_version") {
		t.Error("GenerateOverrideConf() should include frontend_version map")
	}
}

func TestGenerateOverrideConf_HasGeoBlock(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.example.com": {
				OverrideIPs: []string{"192.168.1.0/24", "10.0.0.1"},
			},
		},
	}

	result := GenerateOverrideConf(cfg)

	if !strings.Contains(result, "geo $override_allowed_test_example_com") {
		t.Error("GenerateOverrideConf() should include geo block for site")
	}

	if !strings.Contains(result, "192.168.1.0/24 1;") {
		t.Error("GenerateOverrideConf() should include override IPs")
	}
}

func TestGenerateOverrideConf_DeterministicOrder(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"zebra.example.com": {},
			"alpha.example.com": {},
			"beta.example.com":  {},
		},
	}

	// Generate multiple times and ensure output is consistent
	result1 := GenerateOverrideConf(cfg)
	result2 := GenerateOverrideConf(cfg)

	if result1 != result2 {
		t.Error("GenerateOverrideConf() should produce deterministic output")
	}

	// Verify sites are in alphabetical order
	alphaIdx := strings.Index(result1, "site: alpha.example.com")
	betaIdx := strings.Index(result1, "site: beta.example.com")
	zebraIdx := strings.Index(result1, "site: zebra.example.com")

	if alphaIdx > betaIdx || betaIdx > zebraIdx {
		t.Error("GenerateOverrideConf() should sort sites alphabetically")
	}
}

func TestGenerateMainConf_HasManagedHeader(t *testing.T) {
	result := GenerateMainConf()

	if !strings.Contains(result, "MANAGED BY SHIPYARD") {
		t.Error("GenerateMainConf() should include managed header")
	}
}

func TestGenerateMainConf_HasIncludeDirectives(t *testing.T) {
	result := GenerateMainConf()

	if !strings.Contains(result, "include /usr/local/etc/nginx/override.conf") {
		t.Error("GenerateMainConf() should include override.conf")
	}

	if !strings.Contains(result, "include /usr/local/etc/nginx/sites-enabled/*.conf") {
		t.Error("GenerateMainConf() should include sites-enabled")
	}
}

func TestGenerateRobotsTxt(t *testing.T) {
	result := GenerateRobotsTxt()

	if !strings.Contains(result, "User-agent: *") {
		t.Error("GenerateRobotsTxt() should include User-agent directive")
	}

	if !strings.Contains(result, "Allow: /") {
		t.Error("GenerateRobotsTxt() should allow all paths")
	}
}

func TestTransformToHTTPS_AddsRedirectBlock(t *testing.T) {
	httpConfig := `server {
    listen 80;
    server_name example.com;
    root /var/www;
}`

	result := TransformToHTTPS(httpConfig, "example.com", "/etc/ssl/cert.pem", "/etc/ssl/key.pem")

	if !strings.Contains(result, "listen 80;") {
		t.Error("TransformToHTTPS() should include HTTP redirect block")
	}

	if !strings.Contains(result, "return 301 https://") {
		t.Error("TransformToHTTPS() should redirect to HTTPS")
	}
}

func TestTransformToHTTPS_AddsSSLDirectives(t *testing.T) {
	httpConfig := `server {
    listen 80;
    server_name example.com;
    root /var/www;
}`

	result := TransformToHTTPS(httpConfig, "example.com", "/etc/ssl/cert.pem", "/etc/ssl/key.pem")

	if !strings.Contains(result, "listen 443 ssl") {
		t.Error("TransformToHTTPS() should change port to 443 with ssl")
	}

	if !strings.Contains(result, "ssl_certificate /etc/ssl/cert.pem") {
		t.Error("TransformToHTTPS() should include ssl_certificate directive")
	}

	if !strings.Contains(result, "ssl_certificate_key /etc/ssl/key.pem") {
		t.Error("TransformToHTTPS() should include ssl_certificate_key directive")
	}

	if !strings.Contains(result, "TLSv1.2 TLSv1.3") {
		t.Error("TransformToHTTPS() should include modern TLS protocols")
	}
}

func TestRenderUserConfig_BasicTemplate(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"example.com": {
				FrontendRoot: "/var/www/example.com",
				APIKey:       "sk-site-test",
				SSLEnabled:   true,
				Backend: &config.BackendConfig{
					JailName:   "example-com",
					JailIP:     "127.0.1.1",
					ListenPort: 8080,
					ProxyPath:  "/api",
					BinaryName: "myapp",
				},
			},
		},
	}

	tmpl := `server {
    listen 80;
    server_name <%.Domain%>;
    root <%.FrontendRoot%>/latest;

    location <%.ProxyPath%>/ {
        proxy_pass http://127.0.0.1:<%.ListenPort%>;
    }
}`

	result, err := RenderUserConfig(tmpl, "example.com", cfg)
	if err != nil {
		t.Fatalf("RenderUserConfig() error = %v", err)
	}

	if !strings.Contains(result, "server_name example.com;") {
		t.Error("RenderUserConfig() should substitute Domain")
	}
	if !strings.Contains(result, "root /var/www/example.com/latest;") {
		t.Error("RenderUserConfig() should substitute FrontendRoot")
	}
	if !strings.Contains(result, "location /api/") {
		t.Error("RenderUserConfig() should substitute ProxyPath")
	}
	if !strings.Contains(result, "proxy_pass http://127.0.0.1:8080;") {
		t.Error("RenderUserConfig() should substitute ListenPort")
	}
}

func TestRenderUserConfig_NoBackend(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"static.com": {
				FrontendRoot: "/var/www/static.com",
				APIKey:       "sk-site-test",
			},
		},
	}

	tmpl := `server {
    server_name <%.Domain%>;
    root <%.FrontendRoot%>/latest;
}`

	result, err := RenderUserConfig(tmpl, "static.com", cfg)
	if err != nil {
		t.Fatalf("RenderUserConfig() error = %v", err)
	}

	if !strings.Contains(result, "server_name static.com;") {
		t.Error("RenderUserConfig() should work without backend")
	}
}

func TestRenderUserConfig_InvalidTemplate(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"test.com": {
				FrontendRoot: "/var/www/test.com",
				APIKey:       "sk-site-test",
			},
		},
	}

	// Invalid template syntax
	_, err := RenderUserConfig("server { <% .Invalid }", "test.com", cfg)
	if err == nil {
		t.Error("RenderUserConfig() should return error for invalid template")
	}
}

func TestRenderUserConfig_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{},
	}

	_, err := RenderUserConfig("server {}", "missing.com", cfg)
	if err == nil {
		t.Error("RenderUserConfig() should return error for missing site")
	}
}

func TestRenderUserConfig_PlainConfig(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{
			"plain.com": {
				FrontendRoot: "/var/www/plain.com",
				APIKey:       "sk-site-test",
			},
		},
	}

	// Config with no template directives should pass through unchanged
	plain := "server {\n    listen 80;\n    server_name plain.com;\n}"
	result, err := RenderUserConfig(plain, "plain.com", cfg)
	if err != nil {
		t.Fatalf("RenderUserConfig() error = %v", err)
	}

	if result != plain {
		t.Errorf("RenderUserConfig() should pass through plain configs unchanged, got %q", result)
	}
}

func TestGetOverrideExample(t *testing.T) {
	example := GetOverrideExample()

	if !strings.Contains(example, "<%.Domain%>") {
		t.Error("GetOverrideExample() should contain Domain template variable")
	}
	if !strings.Contains(example, "<%.FrontendRoot%>") {
		t.Error("GetOverrideExample() should contain FrontendRoot template variable")
	}
	if !strings.Contains(example, "<%.AcmeWebroot%>") {
		t.Error("GetOverrideExample() should contain AcmeWebroot template variable")
	}
	if !strings.Contains(example, "Available template variables") {
		t.Error("GetOverrideExample() should contain documentation of available variables")
	}
}

func TestGetIndent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"    listen 80;", "    "},
		{"\tlisten 80;", "\t"},
		{"listen 80;", ""},
		{"", ""},
		{"  \t  mixed;", "  \t  "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := getIndent(tt.input)
			if got != tt.want {
				t.Errorf("getIndent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
