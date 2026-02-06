//go:build !freebsd

package service

// enableService is a no-op on non-FreeBSD platforms
func enableService(name string) error {
	return nil
}

// disableService is a no-op on non-FreeBSD platforms
func disableService(name string) error {
	return nil
}

// startService is a no-op on non-FreeBSD platforms
func startService(name string) error {
	return nil
}

// stopService is a no-op on non-FreeBSD platforms
func stopService(name string) {
}

// restartService is a no-op on non-FreeBSD platforms
func restartService(name string) error {
	return nil
}

// checkService always returns true on non-FreeBSD platforms
func checkService(name string) bool {
	return true
}
