package cmd

import "fmt"

// PrintVersion prints the version info
func PrintVersion(version, commit string) {
	fmt.Printf("shipyard version %s (commit %s)\n", version, commit)
}
