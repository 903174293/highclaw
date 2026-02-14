package main

import (
	"fmt"
	"os"

	"github.com/highclaw/highclaw/internal/cli"
)

// Build-time variables injected via ldflags.
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	cli.SetBuildInfo(Version, BuildDate, GitCommit)

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[highclaw] %v\n", err)
		os.Exit(1)
	}
}
