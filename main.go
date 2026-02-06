package main

import (
	"fmt"
	"os"

	"github.com/lachierussell/shipyard/cmd"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: shipyard <command>\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  serve       - Start the HTTP server\n")
		fmt.Fprintf(os.Stderr, "  bootstrap   - Bootstrap shipyard onto FreeBSD\n")
		fmt.Fprintf(os.Stderr, "  rollback    - Restore previous binary after failed update\n")
		fmt.Fprintf(os.Stderr, "  version     - Print version info\n")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		if err := cmd.Serve(Version, Commit); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "bootstrap":
		if err := cmd.Bootstrap(Version, Commit); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "rollback":
		if err := cmd.Rollback(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		cmd.PrintVersion(Version, Commit)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
