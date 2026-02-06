//go:build freebsd

package service

import (
	"fmt"
	"os/exec"
)

// enableService enables a service on FreeBSD using sysrc
func enableService(name string) error {
	cmd := exec.Command("sysrc", name+"_enable=YES")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}
	return nil
}

// disableService disables a service on FreeBSD using sysrc
func disableService(name string) error {
	cmd := exec.Command("sysrc", name+"_enable=NO")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("disable service: %w", err)
	}
	return nil
}

// startService starts a service on FreeBSD
func startService(name string) error {
	cmd := exec.Command("service", name, "start")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

// stopService stops a service on FreeBSD
func stopService(name string) {
	cmd := exec.Command("service", name, "stop")
	cmd.Run() // Ignore error - service might not be running
}

// restartService restarts a service on FreeBSD
func restartService(name string) error {
	cmd := exec.Command("service", name, "restart")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restart service: %w", err)
	}
	return nil
}

// checkService checks if a service is running on FreeBSD
func checkService(name string) bool {
	cmd := exec.Command("service", name, "status")
	return cmd.Run() == nil
}
