package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lachierussell/shipyard/nginx"
)

// Bootstrap sets up shipyard on a fresh FreeBSD system
func Bootstrap(version, commit string) error {
	slog.Info("starting bootstrap", "version", version, "commit", commit)

	// Paths (using /usr/local for macOS compatibility, would be / on FreeBSD)
	binaryPath := "/usr/local/bin/shipyard"
	configDir := "/usr/local/etc/shipyard"
	nginxConfPath := "/tmp/shipyard-test/etc/nginx.conf" // For testing on macOS

	// 1. Copy binary
	slog.Info("bootstrap: installing binary", "path", binaryPath)
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("read binary: %w", err)
	}

	if err := os.WriteFile(binaryPath, data, 0755); err != nil {
		slog.Warn("bootstrap: skipped binary install", "error", err)
	} else {
		slog.Info("bootstrap: binary installed")
	}

	// 2. Create config directory
	slog.Info("bootstrap: creating config directory", "path", configDir)
	os.MkdirAll(configDir, 0755)

	// 3. Generate nginx main config
	slog.Info("bootstrap: generating nginx main config", "path", nginxConfPath)
	nginxMain := nginx.GenerateMainConf()
	os.WriteFile(nginxConfPath, []byte(nginxMain), 0644)

	// 4. Create example config
	configPath := filepath.Join(configDir, "shipyard.toml")
	slog.Info("bootstrap: creating example config", "path", configPath)
	exampleConfig := `[server]
listen_addr = "0.0.0.0:8443"
log_file    = "/var/log/shipyard/shipyard.log"

[nginx]
binary_path     = "/usr/local/sbin/nginx"
main_conf_path  = "/usr/local/etc/nginx/nginx.conf"
sites_available = "/usr/local/etc/nginx/sites-available"
sites_enabled   = "/usr/local/etc/nginx/sites-enabled"
override_conf   = "/usr/local/etc/nginx/override.conf"

[jail]
base_dir       = "/var/jails"
jail_conf_path = "/etc/jail.conf"
freebsd_version = "14.3-RELEASE"
tarball_cache  = "/var/cache/shipyard/base.txz"
ip_base        = "127.0.1"

[health]
poll_interval     = "15s"
failure_threshold = 3

[self]
binary_path = "/usr/local/bin/shipyard"
pid_file    = "/var/run/shipyard.pid"
config_dir  = "/usr/local/etc/shipyard"

admin_keys = [
    "sk-admin-change-me-to-a-real-key",
]

[[site]]
domain        = "example.com"
frontend_root = "/usr/local/www/example.com"
api_key       = "sk-live-example-change-me"
override_ips  = ["127.0.0.1"]
`

	os.WriteFile(configPath, []byte(exampleConfig), 0644)

	// 5. Create rc.d script
	rcdPath := "/usr/local/etc/rc.d/shipyard"
	slog.Info("bootstrap: creating rc.d script", "path", rcdPath)
	rcdScript := `#!/bin/sh
# PROVIDE: shipyard
# REQUIRE: networking syslog
# KEYWORD: shutdown
#
# MANAGED BY SHIPYARD BOOTSTRAP

. /etc/rc.subr

name="shipyard"
rcvar="${name}_enable"
pidfile="/var/run/shipyard.pid"

command="/usr/sbin/daemon"
command_args="-P ${pidfile} -r -R 5 -f -l daemon -T shipyard /usr/local/bin/shipyard serve"

load_rc_config $name
: ${shipyard_enable:=no}

run_rc_command "$1"
`

	os.WriteFile(rcdPath, []byte(rcdScript), 0755)

	// 6. Summary
	slog.Info("bootstrap complete",
		"config", configPath,
		"binary", binaryPath,
		"rcd", rcdPath,
	)

	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the configuration: sudo vi " + configPath)
	fmt.Println("2. Enable shipyard: sudo sysrc shipyard_enable=YES")
	fmt.Println("3. Start shipyard: sudo service shipyard start")
	fmt.Println("4. Check status: sudo service shipyard status")

	return nil
}
