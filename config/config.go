package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server    ServerConfig          `toml:"server"`
	Nginx     NginxConfig           `toml:"nginx"`
	Jail      JailConfig            `toml:"jail"`
	Health    HealthConfig          `toml:"health"`
	Self      SelfConfig            `toml:"self"`
	AdminKeys []string              `toml:"admin_keys"`
	Site      map[string]SiteConfig `toml:"site"`

	// Runtime fields (not serialized)
	path string
	mu   sync.RWMutex
}

type ServerConfig struct {
	ListenAddr string `toml:"listen_addr"`
	LogFile    string `toml:"log_file"`
	LogLevel   string `toml:"log_level"`
	TLSCert    string `toml:"tls_cert"`
	TLSKey     string `toml:"tls_key"`
}

type NginxConfig struct {
	BinaryPath      string `toml:"binary_path"`
	MainConfPath    string `toml:"main_conf_path"`
	SitesAvailable  string `toml:"sites_available"`
	SitesEnabled    string `toml:"sites_enabled"`
	OverrideConf    string `toml:"override_conf"`
}

type JailConfig struct {
	BaseDir        string `toml:"base_dir"`
	JailConfPath   string `toml:"jail_conf_path"`
	FreeBSDVersion string `toml:"freebsd_version"`
	TarballCache   string `toml:"tarball_cache"`
	IPBase         string `toml:"ip_base"`
}

type HealthConfig struct {
	PollInterval     time.Duration `toml:"poll_interval"`
	FailureThreshold int           `toml:"failure_threshold"`
	HealthPath       string        `toml:"health_path"`
}

type SelfConfig struct {
	BinaryPath string `toml:"binary_path"`
	PidFile    string `toml:"pid_file"`
	ConfigDir  string `toml:"config_dir"`
}

type SiteConfig struct {
	FrontendRoot string         `toml:"frontend_root"`
	APIKey       string         `toml:"api_key"`
	OverrideIPs  []string       `toml:"override_ips"`
	Backend      *BackendConfig `toml:"backend"`
	SSLEnabled   bool           `toml:"ssl_enabled"` // Enable HTTPS with auto-generated Let's Encrypt certs
}

// HasFrontend returns true if the site serves a frontend (has a frontend_root configured).
func (s SiteConfig) HasFrontend() bool {
	return s.FrontendRoot != ""
}

// IsBackendOnly returns true if the site is a backend-only service with no frontend.
func (s SiteConfig) IsBackendOnly() bool {
	return s.Backend != nil && s.FrontendRoot == ""
}

type BackendConfig struct {
	JailName   string `toml:"jail_name"`
	JailIP     string `toml:"jail_ip"`
	ListenPort int    `toml:"listen_port"`
	ProxyPath  string `toml:"proxy_path"`
	BinaryName string `toml:"binary_name"`
}

// Load reads and parses a TOML config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.path = path
	return &cfg, cfg.Validate()
}

// GetSite returns a site config by name, or an error if not found
func (c *Config) GetSite(name string) (*SiteConfig, error) {
	site, ok := c.Site[name]
	if !ok {
		return nil, fmt.Errorf("site not found: %s", name)
	}
	return &site, nil
}

// GetSiteWithBackend returns a site config that has a backend configured
func (c *Config) GetSiteWithBackend(name string) (*SiteConfig, error) {
	site, err := c.GetSite(name)
	if err != nil {
		return nil, err
	}
	if site.Backend == nil {
		return nil, fmt.Errorf("site %s has no backend config", name)
	}
	return site, nil
}

// Validate checks that required fields are present and valid
func (c *Config) Validate() error {
	if c.Server.ListenAddr == "" {
		return fmt.Errorf("server.listen_addr is required")
	}
	if len(c.AdminKeys) == 0 {
		return fmt.Errorf("admin_keys must not be empty")
	}
	if c.Nginx.BinaryPath == "" || c.Nginx.MainConfPath == "" || c.Nginx.SitesAvailable == "" || c.Nginx.SitesEnabled == "" {
		return fmt.Errorf("nginx config paths are required")
	}
	if c.Jail.BaseDir == "" || c.Jail.JailConfPath == "" {
		return fmt.Errorf("jail config paths are required")
	}
	if len(c.Site) == 0 {
		return fmt.Errorf("at least one site must be configured")
	}
	for domain, site := range c.Site {
		if site.FrontendRoot == "" && site.Backend == nil {
			return fmt.Errorf("site %q: frontend_root is required (or configure a backend for backend-only mode)", domain)
		}
		if site.APIKey == "" {
			return fmt.Errorf("site %q: api_key is required", domain)
		}
	}
	return nil
}

// Save writes the config back to disk
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.path == "" {
		return fmt.Errorf("config path not set")
	}

	f, err := os.Create(c.path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

// AddSite adds a new site to the config and saves it
func (c *Config) AddSite(name string, site SiteConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Site[name]; exists {
		return fmt.Errorf("site %q already exists", name)
	}

	if c.Site == nil {
		c.Site = make(map[string]SiteConfig)
	}
	c.Site[name] = site

	// Save without lock (we already hold it)
	if c.path == "" {
		return fmt.Errorf("config path not set")
	}

	f, err := os.Create(c.path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

// GenerateAPIKey generates a secure random API key with the given prefix
func GenerateAPIKey(prefix string) (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return prefix + hex.EncodeToString(bytes), nil
}

// RemoveSite removes a site from the config and saves it
func (c *Config) RemoveSite(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Site[name]; !exists {
		return fmt.Errorf("site %q does not exist", name)
	}

	delete(c.Site, name)

	// Save without lock (we already hold it)
	if c.path == "" {
		return fmt.Errorf("config path not set")
	}

	f, err := os.Create(c.path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

// GetSiteByDomain finds a site by its domain name (domain is the key)
func (c *Config) GetSiteByDomain(domain string) (*SiteConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	site, ok := c.Site[domain]
	if !ok {
		return nil, false
	}
	return &site, true
}

// NextJailIP allocates the next available jail IP based on ip_base config.
// Returns IP in format "{ip_base}.{next_number}" e.g. "127.0.1.5"
func (c *Config) NextJailIP() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ipBase := c.Jail.IPBase
	if ipBase == "" {
		ipBase = "127.0.1"
	}

	// Find highest used IP number
	maxNum := 0
	for _, site := range c.Site {
		if site.Backend != nil && site.Backend.JailIP != "" {
			// Parse the last octet from the IP
			ip := site.Backend.JailIP
			lastDot := len(ip) - 1
			for lastDot >= 0 && ip[lastDot] != '.' {
				lastDot--
			}
			if lastDot >= 0 {
				var num int
				fmt.Sscanf(ip[lastDot+1:], "%d", &num)
				if num > maxNum {
					maxNum = num
				}
			}
		}
	}

	// Return next IP
	return fmt.Sprintf("%s.%d", ipBase, maxNum+1)
}
