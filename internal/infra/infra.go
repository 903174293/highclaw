// Package infra provides low-level infrastructure utilities.
package infra

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// RuntimeInfo contains information about the current runtime environment.
type RuntimeInfo struct {
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	NumCPU    int    `json:"numCPU"`
}

// GetRuntimeInfo returns information about the current runtime.
func GetRuntimeInfo() RuntimeInfo {
	return RuntimeInfo{
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
	}
}

// IsTruthyEnv checks if an environment variable is set to a truthy value.
func IsTruthyEnv(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "1" || v == "true" || v == "yes"
}

// PrintBanner prints the HighClaw startup banner.
func PrintBanner(version string) {
	fmt.Println()
	fmt.Println("  🦀 HighClaw — Personal AI Assistant Gateway")
	fmt.Printf("     version: %s\n", version)
	fmt.Printf("     runtime: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Println()
}
