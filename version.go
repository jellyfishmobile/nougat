package main

import (
	"fmt"
	"os"
	"strings"
)

// Injected at link time, e.g. make build or:
// go build -ldflags "-X main.version=1.0.0 -X main.buildNumber=42 -X main.buildTime=2026-04-08T12:00:00Z" .
var (
	version     = "0.0.0-dev"
	buildNumber = "0"
	buildTime   = "unknown"
)

func versionLine() string {
	v := strings.TrimSpace(version)
	b := strings.TrimSpace(buildNumber)
	t := strings.TrimSpace(buildTime)
	if v == "" {
		v = "0.0.0-dev"
	}
	if b == "" {
		b = "0"
	}
	return fmt.Sprintf("version %s · build %s · built %s", v, b, t)
}

func printVersion() {
	fmt.Fprintln(os.Stdout, "nougat", versionLine())
}

func printBuildInfo() {
	line := versionLine()
	if supportsANSI() {
		line = ansiGray + line + ansiReset
	}
	fmt.Fprintf(os.Stdout, "  %s\n", line)
}
