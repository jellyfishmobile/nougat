package main

import (
	"fmt"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiWhite  = "\033[97m"
)

const nougatBannerArt = `    ███╗   ██╗ ██████╗ ██╗   ██╗ ██████╗  █████╗ ████████╗
    ████╗  ██║██╔═══██╗██║   ██║██╔════╝ ██╔══██╗╚══██╔══╝
    ██╔██╗ ██║██║   ██║██║   ██║██║  ███╗███████║   ██║
    ██║╚██╗██║██║   ██║██║   ██║██║   ██║██╔══██║   ██║
    ██║ ╚████║╚██████╔╝╚██████╔╝╚██████╔╝██║  ██║   ██║
    ╚═╝  ╚═══╝ ╚═════╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═╝   ╚═╝`

// printBanner prints the colored NOUGAT ASCII art (no tagline).
func printBanner() {
	banner := nougatBannerArt
	if supportsANSI() {
		lines := strings.Split(banner, "\n")
		for i := range lines {
			lines[i] = ansiCyan + ansiBold + lines[i] + ansiReset
		}
		banner = strings.Join(lines, "\n")
	}
	fmt.Fprintf(os.Stdout, "\n%s\n", banner)
}

func printWelcome() {
	printBanner()
	tagline := "Claude task runner · HTTP API"
	if supportsANSI() {
		tagline = ansiDim + "Claude task runner" + ansiReset + " · " + ansiGray + "HTTP API" + ansiReset
	}
	fmt.Fprintf(os.Stdout, "  %s\n\n", tagline)
}

func printServerStatus(db *bolt.DB, addr, dbPath, logDir string) {
	title := "Status"
	if supportsANSI() {
		title = ansiBold + ansiWhite + "Status" + ansiReset
	}
	fmt.Fprintf(os.Stdout, "  %s\n", title)
	addrDisp := addr
	if supportsANSI() {
		addrDisp = ansiGreen + addr + ansiReset
	}
	fmt.Fprintf(os.Stdout, "  %s Listen     %s\n", dimLabel("│"), addrDisp)
	fmt.Fprintf(os.Stdout, "  %s Database   %s\n", dimLabel("│"), dbPath)
	fmt.Fprintf(os.Stdout, "  %s Job logs   %s\n", dimLabel("│"), logDir)
	acc := accessLogFile(logDir)
	if acc != "" {
		fmt.Fprintf(os.Stdout, "  %s Access log %s\n", dimLabel("│"), acc)
	}

	total, byStatus, err := jobStats(db)
	if err != nil {
		errMsg := "(error reading stats)"
		if supportsANSI() {
			errMsg = ansiYellow + errMsg + ansiReset
		}
		fmt.Fprintf(os.Stdout, "  %s Jobs       %s\n", dimLabel("│"), errMsg)
	} else {
		fmt.Fprintf(os.Stdout, "  %s Jobs       %d total", dimLabel("│"), total)
		order := []JobStatus{StatusPending, StatusRunning, StatusCompleted, StatusFailed, StatusTimeout}
		parts := []string{}
		for _, st := range order {
			if n := byStatus[st]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", st, n))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(os.Stdout, " (%s)", strings.Join(parts, " "))
		}
		fmt.Fprintln(os.Stdout)
	}

	ep := "Endpoints"
	if supportsANSI() {
		ep = ansiBold + ansiWhite + "Endpoints" + ansiReset
	}
	fmt.Fprintf(os.Stdout, "\n  %s\n", ep)
	fmt.Fprintf(os.Stdout, "  %s POST  /run-task   accept task\n", dimLabel("│"))
	fmt.Fprintf(os.Stdout, "  %s GET   /jobs/{id} job status & output\n", dimLabel("│"))
	fmt.Fprintf(os.Stdout, "  %s GET   /health    liveness\n", dimLabel("│"))

	fmt.Fprintln(os.Stdout)
	if supportsANSI() {
		fmt.Fprintf(os.Stdout, "  %sUsage log%s (requests below)\n\n", ansiDim, ansiReset)
	} else {
		fmt.Fprintln(os.Stdout, "  Usage log (requests below)")
		fmt.Fprintln(os.Stdout)
	}
}

func dimLabel(s string) string {
	if supportsANSI() {
		return ansiGray + s + ansiReset
	}
	return s
}

func supportsANSI() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}
